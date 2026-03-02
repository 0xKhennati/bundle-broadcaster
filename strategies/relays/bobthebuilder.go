package relays

import (
	"github.com/bundle-broadcaster/strategies"
)

type BobthebuilderBuilder struct{}

func (b *BobthebuilderBuilder) BuildRequest(bundle *strategies.IncomingBundle) (string, interface{}, error) {
	return "eth_sendBundle", basePayload(bundle), nil
}
