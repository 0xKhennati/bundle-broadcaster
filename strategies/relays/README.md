# Relay-Specific Builders

Each relay has its own `BuildRequest` function. Relays are identified by `name` in config. **A relay must be registered to work** – config only has `name` and `url`.

## Config Format

```json
"relays": [
  { "name": "titanbuilder", "url": "https://eu.rpc.titanbuilder.xyz" },
  { "name": "flashbots", "url": "https://relay.flashbots.net" }
]
```

## Adding a New Relay

1. Create `strategies/relays/<relayname>.go`:

```go
package relays

import (
	"github.com/0xKhennati/bundle-broadcaster/strategies"
)

type MyrelayBuilder struct{}

func (b *MyrelayBuilder) BuildRequest(bundle *strategies.IncomingBundle) (string, interface{}, error) {
	return "eth_sendBundle", basePayload(bundle), nil
}
```

2. Register in `strategies/relays/register.go`:

```go
strategies.RegisterRelay("myrelay", &MyrelayBuilder{})
```

3. Add to config.json:

```json
{ "name": "myrelay", "url": "https://myrelay.example.com" }
```

Relays in config without a registered builder are skipped with a warning.
