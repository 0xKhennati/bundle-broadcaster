# Relay-Specific Builders

Each relay has its own `BuildRequest` function that formats the bundle payload for that relay's API.

## Adding a New Relay

1. Create `strategies/relays/<relayname>.go`:

```go
package relays

import (
	"github.com/bundle-broadcaster/strategies"
)

type MyrelayBuilder struct{}

func (b *MyrelayBuilder) BuildRequest(bundle *strategies.IncomingBundle) (string, interface{}, error) {
	// Custom payload formatting for this relay
	return "eth_sendBundle", basePayload(bundle), nil
}
```

2. Register in `strategies/relays/register.go`:

```go
strategies.RegisterRelay("myrelay", &MyrelayBuilder{})
```

## Fallback

Relays not in the registry use the type-based strategy from config (`default`, `sendEndOfBlockBundle`, `unified_bundle`).
