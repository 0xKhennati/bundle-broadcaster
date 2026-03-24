package main

import (
	"context"
	"net/http"
	"sync"

	"github.com/0xKhennati/bundle-broadcaster/strategies"
	"github.com/rs/zerolog"
)

func newSharedHTTPClient() *http.Client {
	transport := &http.Transport{
		MaxIdleConns:          maxIdleConns,
		MaxIdleConnsPerHost:   maxIdleConnsHost,
		IdleConnTimeout:       idleConnTimeout,
		DisableKeepAlives:     false,
		ForceAttemptHTTP2:     true,
		ExpectContinueTimeout: 0,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   httpTimeout,
	}
}

type RelayManager struct {
	clients   []*RelayClient
	refund    RefundConfig
	simulator *Simulator
	tracker   *Tracker
	logger    zerolog.Logger
}

func NewRelayManager(cfg *Config, signer *Signer, httpClient *http.Client, logger zerolog.Logger) *RelayManager {
	// Initialize tracker first so relay clients can reference it.
	var tracker *Tracker
	if cfg.Tracking.Enabled && len(cfg.Tracking.Builders) > 0 {
		tracker = NewTracker(cfg.Tracking, logger)
		logger.Info().
			Str("dir", cfg.Tracking.ResolvedDir()).
			Strs("builders", cfg.Tracking.Builders).
			Msg("bundle tracking enabled")
	}

	clients := make([]*RelayClient, 0, len(cfg.Relays))
	for _, relay := range cfg.Relays {
		builder := strategies.GetRelayBuilder(relay.Name)
		if builder == nil {
			logger.Warn().Str("relay", relay.Name).Str("url", relay.ResolvedURL()).
				Msg("relay not registered, skipping - add builder in strategies/relays/ and register in register.go")
			continue
		}
		client := NewRelayClient(relay, builder, signer, httpClient, logger)
		// Attach tracker only for the builders the user wants to track.
		if tracker != nil && cfg.Tracking.IsTracked(relay.Name) {
			client.tracker = tracker
		}
		clients = append(clients, client)
	}

	var sim *Simulator
	if cfg.Simulate.Enabled {
		sim = NewSimulator(cfg.Simulate, signer, httpClient, logger)
		if tracker != nil {
			sim.tracker = tracker
		}
		logger.Info().Str("url", cfg.Simulate.ResolvedURL()).Msg("bundle simulation enabled")
	}

	return &RelayManager{
		clients:   clients,
		refund:    cfg.Refund,
		simulator: sim,
		tracker:   tracker,
		logger:    logger,
	}
}

// Shutdown flushes the tracker to disk and releases its resources.
// Call this during graceful shutdown before the process exits.
func (m *RelayManager) Shutdown() {
	if m.tracker != nil {
		m.tracker.Stop()
	}
}

// applyRefundDefaults returns a shallow copy of bundle with any unset refund
// fields filled in from the broadcaster-level RefundConfig. Bot-supplied values
// always take precedence; the config is only the fallback.
func (m *RelayManager) applyRefundDefaults(bundle *strategies.IncomingBundle) *strategies.IncomingBundle {
	r := m.refund
	// Fast path: no config-level refund defined at all.
	if r.Percent == nil && r.Recipient == "" && len(r.TxHashes) == 0 && !r.DelayedRefund && r.RefundIdentity == "" {
		return bundle
	}
	// Shallow copy so we never mutate the caller's struct.
	b := *bundle
	if b.RefundPercent == nil && r.Percent != nil {
		b.RefundPercent = r.Percent
	}
	if b.RefundRecipient == "" && r.Recipient != "" {
		b.RefundRecipient = r.Recipient
	}
	if len(b.RefundTxHashes) == 0 && len(r.TxHashes) > 0 {
		b.RefundTxHashes = r.TxHashes
	}
	if !b.DelayedRefund && r.DelayedRefund {
		b.DelayedRefund = r.DelayedRefund
	}
	if b.RefundIdentity == "" && r.RefundIdentity != "" {
		b.RefundIdentity = r.RefundIdentity
	}
	return &b
}

func (m *RelayManager) Broadcast(ctx context.Context, bundle *strategies.IncomingBundle) {
	bundle = m.applyRefundDefaults(bundle)

	// Fire simulation in background before fanning out — zero added latency.
	if m.simulator != nil {
		m.simulator.SimulateAsync(bundle)
	}

	var wg sync.WaitGroup
	for _, client := range m.clients {
		wg.Add(1)
		go func(c *RelayClient) {
			defer wg.Done()
			c.Broadcast(ctx, bundle)
		}(client)
	}
	wg.Wait()
}

func (m *RelayManager) WarmConnections(ctx context.Context) {
	var wg sync.WaitGroup
	for _, client := range m.clients {
		wg.Add(1)
		go func(c *RelayClient) {
			defer wg.Done()
			c.WarmConnections(ctx)
		}(client)
	}
	wg.Wait()
}
