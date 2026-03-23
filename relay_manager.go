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
	clients []*RelayClient
	refund  RefundConfig
	logger  zerolog.Logger
}

func NewRelayManager(cfg *Config, signer *Signer, httpClient *http.Client, logger zerolog.Logger) *RelayManager {
	clients := make([]*RelayClient, 0, len(cfg.Relays))
	for _, relay := range cfg.Relays {
		builder := strategies.GetRelayBuilder(relay.Name)
		if builder == nil {
			logger.Warn().Str("relay", relay.Name).Str("url", relay.ResolvedURL()).
				Msg("relay not registered, skipping - add builder in strategies/relays/ and register in register.go")
			continue
		}
		client := NewRelayClient(relay, builder, signer, httpClient, logger)
		clients = append(clients, client)
	}
	return &RelayManager{
		clients: clients,
		refund:  cfg.Refund,
		logger:  logger,
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
