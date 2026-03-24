package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
)

// BundleRecord is one line appended to a builder's JSONL tracking file.
// Fields are intentionally flat so you can paste them directly when
// contacting a builder to ask why a bundle was not included.
type BundleRecord struct {
	BundleID    string    `json:"bundle_id"`
	Builder     string    `json:"builder"`
	BundleHash  string    `json:"bundle_hash"`
	LastTxHash  string    `json:"last_tx_hash,omitempty"` // hash of the last tx (the backrun tx)
	TargetBlock uint64    `json:"target_block"`
	Timestamp   time.Time `json:"timestamp"`

	// Simulation fields — populated once eth_callBundle completes.
	// Empty when simulation is disabled or arrives after the flush deadline.
	CoinbaseDiffETH string `json:"coinbase_diff_eth,omitempty"`
	EthToBuilderETH string `json:"eth_to_builder_eth,omitempty"`
	GasFeesETH      string `json:"gas_fees_eth,omitempty"`
	TotalGasUsed    uint64 `json:"total_gas_used,omitempty"`
	BundleGasPrice  string `json:"bundle_gas_price,omitempty"`
	SimStateBlock   uint64 `json:"sim_state_block,omitempty"`

	// Full decoded transaction fields for every tx in the bundle.
	Transactions []TxDetail `json:"transactions,omitempty"`
}

// TxDetail holds all decoded fields of one signed transaction in a bundle.
type TxDetail struct {
	Hash                 string `json:"hash"`
	From                 string `json:"from"`
	To                   string `json:"to,omitempty"`          // empty on contract creation
	Nonce                uint64 `json:"nonce"`
	Gas                  uint64 `json:"gas"`
	// Gas price fields — only one set is populated depending on tx type.
	GasPrice             string `json:"gas_price,omitempty"`              // legacy / type-0
	MaxFeePerGas         string `json:"max_fee_per_gas,omitempty"`        // EIP-1559 / type-2
	MaxPriorityFeePerGas string `json:"max_priority_fee_per_gas,omitempty"` // EIP-1559 / type-2
	Value                string `json:"value_eth"`   // ETH, human-readable (e.g. "0.050000000")
	ValueWei             string `json:"value_wei"`   // wei as decimal string
	Data                 string `json:"data"`        // full calldata hex (0x-prefixed)
	Type                 uint8  `json:"type"`        // 0=legacy 1=accessList 2=dynamicFee
	ChainID              string `json:"chain_id,omitempty"`
}

// HashEvent is emitted by a RelayClient when it receives a bundle hash from a builder.
type HashEvent struct {
	BundleID     string
	Builder      string
	BundleHash   string
	TargetBlock  uint64
	LastTxHash   string     // hash of the last transaction in the bundle (the backrun tx)
	Transactions []TxDetail // all decoded transactions in submission order
}

// SimEvent is emitted by the Simulator when eth_callBundle completes successfully.
type SimEvent struct {
	BundleID        string
	CoinbaseDiffETH string
	EthToBuilderETH string
	GasFeesETH      string
	TotalGasUsed    uint64
	BundleGasPrice  string
	StateBlock      uint64
}

// Tracker collects bundle hashes from tracked builders and correlates them with
// simulation results. All file I/O runs inside a single dedicated goroutine so
// broadcasting is never blocked.
type Tracker struct {
	cfg    TrackingConfig
	hashCh chan HashEvent
	simCh  chan SimEvent
	doneCh chan struct{}
	logger zerolog.Logger
}

func NewTracker(cfg TrackingConfig, logger zerolog.Logger) *Tracker {
	t := &Tracker{
		cfg:    cfg,
		hashCh: make(chan HashEvent, 1024),
		simCh:  make(chan SimEvent, 1024),
		doneCh: make(chan struct{}),
		logger: logger,
	}
	go t.run()
	return t
}

// RecordHash enqueues a bundle hash received from a builder.
// Non-blocking — drops the event if the channel is full.
func (t *Tracker) RecordHash(e HashEvent) {
	select {
	case t.hashCh <- e:
	default:
		t.logger.Warn().Str("bundle_id", e.BundleID).Str("builder", e.Builder).
			Msg("[tracker] channel full, dropping hash event")
	}
}

// RecordSim enqueues a simulation result so it can be correlated with bundle hashes.
// Non-blocking — silently drops if channel is full (sim data is best-effort).
func (t *Tracker) RecordSim(e SimEvent) {
	select {
	case t.simCh <- e:
	default:
	}
}

// Stop flushes all pending records to disk and closes the tracker goroutine.
func (t *Tracker) Stop() {
	close(t.doneCh)
}

// --- internal types ---

type pendingEntry struct {
	record     BundleRecord
	receivedAt time.Time
}

type simCacheEntry struct {
	CoinbaseDiffETH string
	EthToBuilderETH string
	GasFeesETH      string
	TotalGasUsed    uint64
	BundleGasPrice  string
	StateBlock      uint64
	receivedAt      time.Time
}

func (t *Tracker) run() {
	// pending holds bundle hash events waiting for a sim result or the flush deadline.
	// Structure: bundleID -> builderName -> *pendingEntry
	pending := make(map[string]map[string]*pendingEntry)

	// simCache caches simulation results for up to 30 seconds so late-arriving
	// hash events can still be enriched.
	simCache := make(map[string]*simCacheEntry)

	// files maps builder name to its open append-only JSONL file.
	files := make(map[string]*os.File)

	dir := t.cfg.ResolvedDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.logger.Error().Err(err).Str("dir", dir).Msg("[tracker] cannot create tracking directory")
		return
	}

	getFile := func(builder string) *os.File {
		if f, ok := files[builder]; ok {
			return f
		}
		path := filepath.Join(dir, builder+".jsonl")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.logger.Error().Err(err).Str("path", path).Msg("[tracker] cannot open tracking file")
			return nil
		}
		files[builder] = f
		t.logger.Info().Str("path", path).Msg("[tracker] opened tracking file")
		return f
	}

	writeRecord := func(r BundleRecord) {
		f := getFile(r.Builder)
		if f == nil {
			return
		}
		line, err := json.Marshal(r)
		if err != nil {
			return
		}
		line = append(line, '\n')
		if _, err := f.Write(line); err != nil {
			t.logger.Error().Err(err).Str("builder", r.Builder).Msg("[tracker] write failed")
		}
	}

	// flush writes all pending entries older than 2 seconds (or all, if force=true).
	flush := func(force bool) {
		deadline := time.Now().Add(-2 * time.Second)
		for bundleID, builders := range pending {
			for builder, entry := range builders {
				if !force && entry.receivedAt.After(deadline) {
					continue
				}
				// Enrich with sim data if available.
				if sim, ok := simCache[bundleID]; ok {
					entry.record.CoinbaseDiffETH = sim.CoinbaseDiffETH
					entry.record.EthToBuilderETH = sim.EthToBuilderETH
					entry.record.GasFeesETH = sim.GasFeesETH
					entry.record.TotalGasUsed = sim.TotalGasUsed
					entry.record.BundleGasPrice = sim.BundleGasPrice
					entry.record.SimStateBlock = sim.StateBlock
				}
				writeRecord(entry.record)
				delete(builders, builder)
			}
			if len(builders) == 0 {
				delete(pending, bundleID)
			}
		}
		// Evict sim cache entries older than 30 seconds.
		evictBefore := time.Now().Add(-30 * time.Second)
		for id, s := range simCache {
			if s.receivedAt.Before(evictBefore) {
				delete(simCache, id)
			}
		}
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	defer func() {
		for _, f := range files {
			f.Close()
		}
	}()

	for {
		select {
		case <-t.doneCh:
			flush(true)
			return

		case e := <-t.hashCh:
			if _, ok := pending[e.BundleID]; !ok {
				pending[e.BundleID] = make(map[string]*pendingEntry)
			}
			record := BundleRecord{
				BundleID:     e.BundleID,
				Builder:      e.Builder,
				BundleHash:   e.BundleHash,
				LastTxHash:   e.LastTxHash,
				TargetBlock:  e.TargetBlock,
				Timestamp:    time.Now().UTC(),
				Transactions: e.Transactions,
			}
			// Pre-fill sim fields if the sim result is already cached
			// (simulation fires before broadcasts so it often arrives first).
			if sim, ok := simCache[e.BundleID]; ok {
				record.CoinbaseDiffETH = sim.CoinbaseDiffETH
				record.EthToBuilderETH = sim.EthToBuilderETH
				record.GasFeesETH = sim.GasFeesETH
				record.TotalGasUsed = sim.TotalGasUsed
				record.BundleGasPrice = sim.BundleGasPrice
				record.SimStateBlock = sim.StateBlock
			}
			pending[e.BundleID][e.Builder] = &pendingEntry{
				record:     record,
				receivedAt: time.Now(),
			}

		case e := <-t.simCh:
			entry := &simCacheEntry{
				CoinbaseDiffETH: e.CoinbaseDiffETH,
				EthToBuilderETH: e.EthToBuilderETH,
				GasFeesETH:      e.GasFeesETH,
				TotalGasUsed:    e.TotalGasUsed,
				BundleGasPrice:  e.BundleGasPrice,
				StateBlock:      e.StateBlock,
				receivedAt:      time.Now(),
			}
			simCache[e.BundleID] = entry
			// Immediately enrich any pending records already waiting for this bundle.
			if builders, ok := pending[e.BundleID]; ok {
				for _, pe := range builders {
					pe.record.CoinbaseDiffETH = e.CoinbaseDiffETH
					pe.record.EthToBuilderETH = e.EthToBuilderETH
					pe.record.GasFeesETH = e.GasFeesETH
					pe.record.TotalGasUsed = e.TotalGasUsed
					pe.record.BundleGasPrice = e.BundleGasPrice
					pe.record.SimStateBlock = e.StateBlock
				}
			}

		case <-ticker.C:
			flush(false)
		}
	}
}
