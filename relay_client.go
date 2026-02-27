package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bundle-broadcaster/strategies"
	"github.com/rs/zerolog"
)

const (
	httpTimeout       = 900 * time.Millisecond
	maxIdleConns      = 1000
	maxIdleConnsHost  = 500
	idleConnTimeout   = 5 * time.Minute
	warmConnPerRelay  = 100
	maxRetries        = 3
)

var jsonEncoderPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

type RelayClient struct {
	client   *http.Client
	signer   *Signer
	relay    RelayConfig
	strategy strategies.RelayStrategy
	logger   zerolog.Logger
}

func NewRelayClient(relay RelayConfig, strategy strategies.RelayStrategy, signer *Signer, client *http.Client, logger zerolog.Logger) *RelayClient {
	return &RelayClient{
		client:   client,
		signer:   signer,
		relay:    relay,
		strategy: strategy,
		logger:   logger,
	}
}

type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

func (c *RelayClient) Broadcast(ctx context.Context, bundle *strategies.IncomingBundle) {
	method, payload, err := c.strategy.BuildRequest(bundle)
	if err != nil {
		c.logger.Error().Err(err).Str("relay", c.relay.Name).Str("bundle_id", bundle.BundleID).Msg("failed to build request")
		BundleFailedTotal.WithLabelValues(c.relay.Name).Inc()
		return
	}

	reqBody := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  []interface{}{payload},
	}

	buf := jsonEncoderPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer jsonEncoderPool.Put(buf)

	enc := json.NewEncoder(buf)
	if err := enc.Encode(reqBody); err != nil {
		c.logger.Error().Err(err).Str("relay", c.relay.Name).Str("bundle_id", bundle.BundleID).Msg("failed to encode request")
		BundleFailedTotal.WithLabelValues(c.relay.Name).Inc()
		return
	}

	bodyBytes := buf.Bytes()
	signature, err := c.signer.Sign(bodyBytes)
	if err != nil {
		c.logger.Error().Err(err).Str("relay", c.relay.Name).Str("bundle_id", bundle.BundleID).Msg("failed to sign request")
		BundleFailedTotal.WithLabelValues(c.relay.Name).Inc()
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.relay.URL, bytes.NewReader(bodyBytes))
	if err != nil {
		c.logger.Error().Err(err).Str("relay", c.relay.Name).Str("bundle_id", bundle.BundleID).Msg("failed to create request")
		BundleFailedTotal.WithLabelValues(c.relay.Name).Inc()
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Flashbots-Signature", signature)
	req.ContentLength = int64(len(bodyBytes))

	bodyCopy := bytes.Clone(bodyBytes)
	start := time.Now()
	var resp *http.Response
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			req, _ = http.NewRequestWithContext(ctx, http.MethodPost, c.relay.URL, bytes.NewReader(bodyCopy))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Flashbots-Signature", signature)
			req.ContentLength = int64(len(bodyCopy))
		}
		resp, lastErr = c.client.Do(req)
		if lastErr == nil {
			break
		}
		if errors.Is(lastErr, context.Canceled) || errors.Is(lastErr, context.DeadlineExceeded) {
			break
		}
		if !isRetryableConnError(lastErr) {
			break
		}
		if attempt < maxRetries-1 {
			c.logger.Debug().Err(lastErr).Str("relay", c.relay.Name).Str("bundle_id", bundle.BundleID).
				Int("attempt", attempt+1).Msg("connection error, retrying with different connection")
		}
	}

	latencyMs := time.Since(start).Milliseconds()
	RelayLatencyMs.WithLabelValues(c.relay.Name).Observe(float64(latencyMs))

	if lastErr != nil {
		c.logger.Error().Err(lastErr).Str("relay", c.relay.Name).Str("bundle_id", bundle.BundleID).
			Int64("latency_ms", latencyMs).Int("attempts", maxRetries).Msg("relay request failed after retries")
		BundleFailedTotal.WithLabelValues(c.relay.Name).Inc()
		return
	}

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		BundleSentTotal.WithLabelValues(c.relay.Name).Inc()
		c.logger.Debug().Str("relay", c.relay.Name).Str("bundle_id", bundle.BundleID).
			Int64("latency_ms", latencyMs).Int("status", resp.StatusCode).Msg("bundle broadcast success")
	} else {
		BundleFailedTotal.WithLabelValues(c.relay.Name).Inc()
		c.logger.Warn().Str("relay", c.relay.Name).Str("bundle_id", bundle.BundleID).
			Int64("latency_ms", latencyMs).Int("status", resp.StatusCode).Msg("relay returned non-success status")
	}
}

func isRetryableConnError(err error) bool {
	if err == nil {
		return false
	}
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return true
	}
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return true
	}
	s := strings.ToLower(err.Error())
	for _, sub := range []string{"connection refused", "connection reset", "broken pipe", "connection timed out", "i/o timeout", "tls handshake", "eof"} {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

var warmReqBody = func() []byte {
	b, _ := json.Marshal(jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      0,
		Method:  "eth_blockNumber",
		Params:  []interface{}{},
	})
	return b
}()

func (c *RelayClient) WarmConnections(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < warmConnPerRelay; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.relay.URL, bytes.NewReader(warmReqBody))
			if err != nil {
				return
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := c.client.Do(req)
			if err != nil {
				return
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}()
	}
	wg.Wait()
	c.logger.Info().Str("relay", c.relay.Name).Int("connections", warmConnPerRelay).Msg("connections pre-warmed")
}
