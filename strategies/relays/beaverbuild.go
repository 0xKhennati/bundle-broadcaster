package relays

import (
	"github.com/bundle-broadcaster/strategies"
)

type BeaverbuildBuilder struct{}

func (b *BeaverbuildBuilder) BuildRequest(bundle *strategies.IncomingBundle) (string, interface{}, error) {
	return "eth_sendBundle", basePayload(bundle), nil
}
