package main

import (
	"context"
	"net/http"
	"sync"

	"github.com/bundle-broadcaster/strategies"
	"github.com/rs/zerolog"
)

func newSharedHTTPClient() *http.Client {
	transport := &http.Transport{
		MaxIdleConns:           maxIdleConns,
		MaxIdleConnsPerHost:    maxIdleConnsHost,
		IdleConnTimeout:        idleConnTimeout,
		DisableKeepAlives:      false,
		ForceAttemptHTTP2:      true,
		ExpectContinueTimeout: 0,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   httpTimeout,
	}
}

type RelayManager struct {
	clients []*RelayClient
	logger  zerolog.Logger
}

func NewRelayManager(cfg *Config, signer *Signer, httpClient *http.Client, logger zerolog.Logger) *RelayManager {
	clients := make([]*RelayClient, 0, len(cfg.Relays))
	for _, relay := range cfg.Relays {
		builder := strategies.GetRelayBuilder(relay.Name)
		if builder == nil {
			builder = strategyForType(relay.Type)
		}
		client := NewRelayClient(relay, builder, signer, httpClient, logger)
		clients = append(clients, client)
	}
	return &RelayManager{
		clients: clients,
		logger:  logger,
	}
}

func strategyForType(relayType string) strategies.RelayStrategy {
	switch relayType {
	case RelayTypeEndOfBlock:
		return strategies.NewEndOfBlockStrategy()
	case RelayTypeUnifiedBundle:
		return strategies.NewUnifiedBundleStrategy()
	default:
		return strategies.NewDefaultBundleStrategy()
	}
}

func (m *RelayManager) Broadcast(ctx context.Context, bundle *strategies.IncomingBundle) {
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
