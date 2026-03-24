package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/0xKhennati/bundle-broadcaster/strategies"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/rs/zerolog"
)

const simulateTimeout = 2 * time.Second

// callBundlePayload is the params object for eth_callBundle.
type callBundlePayload struct {
	Txs              []string `json:"txs"`
	BlockNumber      string   `json:"blockNumber"`
	StateBlockNumber string   `json:"stateBlockNumber"`
}

// callBundleTxResult is one transaction's result inside the eth_callBundle response.
type callBundleTxResult struct {
	TxHash           string `json:"txHash"`
	GasUsed          uint64 `json:"gasUsed"`
	FromAddress      string `json:"fromAddress"`
	ToAddress        string `json:"toAddress"`
	CoinbaseDiff     string `json:"coinbaseDiff"`
	EthSentToCoinbase string `json:"ethSentToCoinbase"`
	Error            string `json:"error"`
	Revert           string `json:"revert"`
	Value            string `json:"value"`
}

// callBundleResult is the top-level result of eth_callBundle.
type callBundleResult struct {
	BundleHash        string               `json:"bundleHash"`
	BundleGasPrice    string               `json:"bundleGasPrice"`
	CoinbaseDiff      string               `json:"coinbaseDiff"`
	EthSentToCoinbase string               `json:"ethSentToCoinbase"`
	GasFees           string               `json:"gasFees"`
	TotalGasUsed      uint64               `json:"totalGasUsed"`
	StateBlockNumber  uint64               `json:"stateBlockNumber"`
	Results           []callBundleTxResult `json:"results"`
}

type callBundleResponse struct {
	Result *callBundleResult      `json:"result"`
	Error  *jsonRPCError          `json:"error"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Simulator fires eth_callBundle against Flashbots in a background goroutine
// and logs the result. It never blocks bundle broadcasting.
type Simulator struct {
	cfg     SimulateConfig
	signer  *Signer
	client  *http.Client
	logger  zerolog.Logger
	tracker *Tracker // optional; when set, sim results are forwarded to the tracker
}

func NewSimulator(cfg SimulateConfig, signer *Signer, httpClient *http.Client, logger zerolog.Logger) *Simulator {
	return &Simulator{
		cfg:    cfg,
		signer: signer,
		client: httpClient,
		logger: logger,
	}
}

// SimulateAsync fires eth_callBundle in a background goroutine.
// Returns immediately — never blocks the caller.
func (s *Simulator) SimulateAsync(bundle *strategies.IncomingBundle) {
	// Copy the slice so the goroutine owns its data.
	txs := make([]string, len(bundle.RawTxs))
	copy(txs, bundle.RawTxs)
	bundleID := bundle.BundleID
	targetBlock := bundle.TargetBlock

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), simulateTimeout)
		defer cancel()

		result, err := s.callBundle(ctx, txs, targetBlock)
		if err != nil {
			s.logger.Warn().
				Str("bundle_id", bundleID).
				Uint64("target_block", targetBlock).
				Err(err).
				Msg("[simulate] eth_callBundle request failed")
			return
		}

		if result.Error != nil {
			s.logger.Warn().
				Str("bundle_id", bundleID).
				Uint64("target_block", targetBlock).
				Int("code", result.Error.Code).
				Str("message", result.Error.Message).
				Msg("[simulate] bundle REJECTED by flashbots")
			return
		}

		if result.Result == nil {
			s.logger.Warn().
				Str("bundle_id", bundleID).
				Uint64("target_block", targetBlock).
				Msg("[simulate] empty result from flashbots")
			return
		}

		r := result.Result

		// Check if any transaction reverted.
		for i, tx := range r.Results {
			if tx.Error != "" {
				s.logger.Warn().
					Str("bundle_id", bundleID).
					Uint64("target_block", targetBlock).
					Int("tx_index", i).
					Str("tx_hash", tx.TxHash).
					Str("error", tx.Error).
					Str("revert", tx.Revert).
					Msg("[simulate] tx REVERTED — bundle will be dropped by builders")
				return
			}
		}

		// All transactions succeeded — log the profit summary.
		coinbaseDiffEth := weiHexToEth(r.CoinbaseDiff)
		ethToBuilderEth := weiHexToEth(r.EthSentToCoinbase)
		gasFeesEth := weiHexToEth(r.GasFees)

		stateBlock := r.StateBlockNumber
		if stateBlock == 0 {
			stateBlock = targetBlock - 1
		}

		// Forward to tracker so it can correlate with builder bundle hashes.
		if s.tracker != nil {
			s.tracker.RecordSim(SimEvent{
				BundleID:        bundleID,
				CoinbaseDiffETH: coinbaseDiffEth,
				EthToBuilderETH: ethToBuilderEth,
				GasFeesETH:      gasFeesEth,
				TotalGasUsed:    r.TotalGasUsed,
				BundleGasPrice:  r.BundleGasPrice,
				StateBlock:      stateBlock,
			})
		}

		isZeroPayment := r.EthSentToCoinbase == "" || r.EthSentToCoinbase == "0x0" || r.EthSentToCoinbase == "0"

		logEvent := s.logger.Info().
			Str("bundle_id", bundleID).
			Uint64("target_block", targetBlock).
			Str("coinbase_diff_eth", coinbaseDiffEth).
			Str("eth_to_builder_eth", ethToBuilderEth).
			Str("gas_fees_eth", gasFeesEth).
			Uint64("total_gas_used", r.TotalGasUsed).
			Str("bundle_gas_price", r.BundleGasPrice)

		if isZeroPayment {
			// Log a Tenderly URL for every tx so the user can inspect why
			// there is no coinbase payment.
			for i, rawTx := range txs {
				tURL := buildTenderlyURL(rawTx, stateBlock)
				if tURL != "" {
					logEvent = logEvent.Str(fmt.Sprintf("tenderly_tx%d", i), tURL)
				}
			}
			logEvent.Msg("[simulate] bundle VALID but pays 0 to builder — open Tenderly links to debug missing coinbase payment")
		} else {
			logEvent.Msg("[simulate] bundle VALID ✓ — if not landing, increase coinbase payment")
		}
	}()
}

func (s *Simulator) callBundle(ctx context.Context, txs []string, targetBlock uint64) (*callBundleResponse, error) {
	payload := callBundlePayload{
		Txs:              txs,
		BlockNumber:      fmt.Sprintf("0x%x", targetBlock),
		StateBlockNumber: "latest",
	}

	reqBody := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "eth_callBundle",
		Params:  []interface{}{payload},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	signature, err := s.signer.Sign(bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.ResolvedURL(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Flashbots-Signature", signature)
	req.ContentLength = int64(len(bodyBytes))

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 65536))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var result callBundleResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal (body=%s): %w", string(respBody), err)
	}
	return &result, nil
}

// buildTenderlyURL decodes a raw signed transaction and returns a Tenderly
// simulator URL pre-filled with all transaction fields, anchored to blockNumber
// for state. Returns "" if the transaction cannot be decoded.
func buildTenderlyURL(rawHex string, blockNumber uint64) string {
	s := strings.TrimPrefix(strings.TrimPrefix(rawHex, "0x"), "0X")
	rawBytes, err := hex.DecodeString(s)
	if err != nil {
		return ""
	}

	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(rawBytes); err != nil {
		return ""
	}

	signer := types.LatestSignerForChainID(tx.ChainId())
	from, err := types.Sender(signer, tx)
	if err != nil {
		return ""
	}

	to := "0x0000000000000000000000000000000000000000"
	if tx.To() != nil {
		to = tx.To().Hex()
	}

	gasPrice := tx.GasPrice()
	if tx.Type() == types.DynamicFeeTxType && tx.GasFeeCap() != nil {
		gasPrice = tx.GasFeeCap()
	}
	if gasPrice == nil {
		gasPrice = big.NewInt(0)
	}

	params := url.Values{}
	params.Set("network", "1")
	params.Set("from", from.Hex())
	params.Set("to", to)
	params.Set("gas", fmt.Sprintf("%d", tx.Gas()))
	params.Set("gasPrice", gasPrice.String())
	params.Set("value", tx.Value().String())
	params.Set("rawFunctionInput", "0x"+hex.EncodeToString(tx.Data()))
	params.Set("block", fmt.Sprintf("%d", blockNumber))

	return "https://dashboard.tenderly.co/simulator/new?" + params.Encode()
}

// weiHexToEth converts a hex wei string like "0x2386f26fc10000" to a
// human-readable ETH string like "0.010000000".
func weiHexToEth(hexWei string) string {
	if hexWei == "" || hexWei == "0x0" || hexWei == "0x" {
		return "0"
	}
	n := new(big.Int)
	s := hexWei
	if len(s) >= 2 && s[:2] == "0x" {
		s = s[2:]
	}
	if _, ok := n.SetString(s, 16); !ok {
		return hexWei
	}
	eth := new(big.Float).Quo(new(big.Float).SetInt(n), big.NewFloat(1e18))
	return fmt.Sprintf("%.9f", eth)
}
