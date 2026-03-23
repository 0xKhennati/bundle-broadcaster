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
	"sync/atomic"
	"time"

	"github.com/0xKhennati/bundle-broadcaster/strategies"
	"github.com/rs/zerolog"
)

const (
	httpTimeout             = 900 * time.Millisecond
	maxIdleConns            = 1000
	maxIdleConnsHost        = 500
	idleConnTimeout         = 5 * time.Minute
	warmConnPerRelay        = 100
	maxRetries              = 3
	defaultCooldownMs       = 1000 // 1 second default 429 backoff
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
	// rateLimitedUntil holds a Unix nanosecond timestamp.
	// While time.Now().UnixNano() < rateLimitedUntil, all broadcasts are skipped.
	rateLimitedUntil atomic.Int64
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

func (c *RelayClient) cooldownMs() int64 {
	if c.relay.RateLimitCooldownMs > 0 {
		return int64(c.relay.RateLimitCooldownMs)
	}
	return defaultCooldownMs
}

func (c *RelayClient) Broadcast(ctx context.Context, bundle *strategies.IncomingBundle) {
	// Skip relay if it is currently in a 429 cooldown window.
	if until := c.rateLimitedUntil.Load(); until > 0 && time.Now().UnixNano() < until {
		c.logger.Debug().Str("relay", c.relay.Name).Str("bundle_id", bundle.BundleID).
			Int64("cooldown_remaining_ms", (until-time.Now().UnixNano())/int64(time.Millisecond)).
			Msg("relay rate-limited, skipping bundle")
		BundleRateLimitedTotal.WithLabelValues(c.relay.Name).Inc()
		return
	}

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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.relay.ResolvedURL(), bytes.NewReader(bodyBytes))
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
			req, _ = http.NewRequestWithContext(ctx, http.MethodPost, c.relay.ResolvedURL(), bytes.NewReader(bodyCopy))
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

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))
	resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		BundleSentTotal.WithLabelValues(c.relay.Name).Inc()
		c.logger.Info().Str("relay", c.relay.Name).
			Int64("latency_ms", latencyMs).Int("status", resp.StatusCode).
			Str("response", string(respBody)).Msg("relay response")
	} else if resp.StatusCode == http.StatusTooManyRequests {
		cooldown := c.cooldownMs()
		c.rateLimitedUntil.Store(time.Now().Add(time.Duration(cooldown) * time.Millisecond).UnixNano())
		BundleRateLimitedTotal.WithLabelValues(c.relay.Name).Inc()
		c.logger.Warn().Str("relay", c.relay.Name).
			Int64("latency_ms", latencyMs).Int64("cooldown_ms", cooldown).
			Msg("relay returned 429, entering cooldown")
	} else {
		BundleFailedTotal.WithLabelValues(c.relay.Name).Inc()
		c.logger.Warn().Str("relay", c.relay.Name).
			Int64("latency_ms", latencyMs).Int("status", resp.StatusCode).
			Str("response", string(respBody)).Msg("relay response")
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
	count := warmConnPerRelay
	if c.relay.WarmupConnections > 0 {
		count = c.relay.WarmupConnections
	} else if c.relay.WarmupConnections < 0 {
		// -1 means skip warmup entirely for this relay.
		c.logger.Info().Str("relay", c.relay.Name).Msg("warmup skipped (disabled in config)")
		return
	}
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.relay.ResolvedURL(), bytes.NewReader(warmReqBody))
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
	c.logger.Info().Str("relay", c.relay.Name).Int("connections", count).Msg("connections pre-warmed")
}
