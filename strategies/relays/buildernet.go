package relays

import (
	"github.com/bundle-broadcaster/strategies"
)

type BuildernetBuilder struct{}

func (b *BuildernetBuilder) BuildRequest(bundle *strategies.IncomingBundle) (string, interface{}, error) {
	return "eth_sendBundle", basePayload(bundle), nil
}
