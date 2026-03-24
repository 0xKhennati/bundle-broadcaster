package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type RelayConfig struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	// WarmupConnections overrides the default number of parallel warmup
	// requests sent to this relay on startup. Set to 0 to skip warmup.
	WarmupConnections int `json:"warmup_connections,omitempty"`
	// RateLimitCooldownMs is how long (in milliseconds) to pause sending
	// to this relay after it responds with HTTP 429. Default: 1000ms.
	RateLimitCooldownMs int `json:"rate_limit_cooldown_ms,omitempty"`
}

// ResolvedURL returns the relay URL with https:// prepended if no scheme is present.
func (r *RelayConfig) ResolvedURL() string {
	url := r.URL
	if url == "" {
		return ""
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return "https://" + url
	}
	return url
}

// RefundConfig defines default refund settings applied to every bundle.
// These are used as fallbacks when the incoming bundle does not specify its own
// refund fields. Bot-supplied values always take priority over these defaults.
// Only relays that support refund params (Titan, Quasar, BuilderNet) will use them;
// Flashbots eth_sendBundle does not support per-bundle refund fields.
type RefundConfig struct {
	// Percent is the percentage (0–99) of the bundle's ETH reward to refund.
	Percent *int `json:"percent,omitempty"`
	// Recipient is the Ethereum address that receives the refund.
	// Defaults to the sender of the first transaction if empty.
	Recipient string `json:"recipient,omitempty"`
	// TxHashes pins which transaction(s) the refund is calculated from.
	// Defaults to the last transaction in the bundle if empty.
	TxHashes []string `json:"tx_hashes,omitempty"`
	// DelayedRefund enables BuilderNet's async refund pipeline.
	DelayedRefund bool `json:"delayed_refund,omitempty"`
	// RefundIdentity overrides the BuilderNet refund recipient address.
	RefundIdentity string `json:"refund_identity,omitempty"`
}

// TrackingConfig controls the optional bundle-hash tracking feature.
// When enabled, every bundle hash returned by a tracked builder is stored in a
// per-builder JSONL file, correlated with the simulation result so you can
// bring concrete data when querying builders about missed inclusions.
type TrackingConfig struct {
	// Enabled turns tracking on or off.
	Enabled bool `json:"enabled"`
	// Dir is the directory where JSONL files are written.
	// Defaults to "tracking/" relative to the working directory.
	Dir string `json:"dir,omitempty"`
	// Builders lists the relay names to track (e.g. ["titanbuilder","beaverbuild","buildernet"]).
	// Only relays whose name appears here will have their bundle hashes recorded.
	Builders []string `json:"builders,omitempty"`
}

func (t *TrackingConfig) ResolvedDir() string {
	if t.Dir != "" {
		return t.Dir
	}
	return "tracking"
}

// IsTracked returns true if the given relay name should be tracked.
func (t *TrackingConfig) IsTracked(builder string) bool {
	for _, b := range t.Builders {
		if b == builder {
			return true
		}
	}
	return false
}

// SimulateConfig controls the optional eth_callBundle simulation feature.
// When enabled, every bundle is simulated against Flashbots in a background
// goroutine and the result is logged. This never delays or blocks broadcasting.
type SimulateConfig struct {
	// Enabled turns the simulation on or off.
	Enabled bool `json:"enabled"`
	// URL is the eth_callBundle endpoint. Defaults to https://relay.flashbots.net.
	URL string `json:"url,omitempty"`
}

func (s *SimulateConfig) ResolvedURL() string {
	if s.URL != "" {
		return s.URL
	}
	return "https://relay.flashbots.net"
}

type Config struct {
	Server     ServerConfig   `json:"server"`
	Auth       AuthConfig     `json:"auth"`
	PrivateKey string         `json:"private_key"`
	LogLevel   string         `json:"log_level"`
	Relays     []RelayConfig  `json:"relays"`
	Refund   RefundConfig   `json:"refund,omitempty"`
	Simulate SimulateConfig `json:"simulate,omitempty"`
	Tracking TrackingConfig `json:"tracking,omitempty"`
}

type AuthConfig struct {
	PasswordHash    string `json:"password_hash"`
	MaxAttempts     int    `json:"max_attempts"`
	LockoutMinutes  int    `json:"lockout_minutes"`
}

type ServerConfig struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

func (s *ServerConfig) Addr() string {
	addr := s.Address
	if addr == "" {
		addr = "0.0.0.0"
	}
	port := s.Port
	if port == 0 {
		port = 1111
	}
	return fmt.Sprintf("%s:%d", addr, port)
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
