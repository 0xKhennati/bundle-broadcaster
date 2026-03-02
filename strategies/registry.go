package strategies

import "sync"

var (
	relayBuilders   = make(map[string]RelayStrategy)
	relayBuildersMu sync.RWMutex
)

// RegisterRelay registers a relay-specific BuildRequest for the given relay name.
func RegisterRelay(name string, builder RelayStrategy) {
	relayBuildersMu.Lock()
	defer relayBuildersMu.Unlock()
	relayBuilders[name] = builder
}

// GetRelayBuilder returns the relay-specific builder, or nil if not registered.
func GetRelayBuilder(name string) RelayStrategy {
	relayBuildersMu.RLock()
	defer relayBuildersMu.RUnlock()
	return relayBuilders[name]
}
