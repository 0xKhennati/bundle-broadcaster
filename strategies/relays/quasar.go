package relays

import (
	"github.com/0xKhennati/bundle-broadcaster/strategies"
)

type QuasarBuilder struct{}

func (b *QuasarBuilder) BuildRequest(bundle *strategies.IncomingBundle) (string, interface{}, error) {
	if bundle.StrategyType == strategies.StrategyTargetBlock || bundle.StrategyType == strategies.StrategyPendingBlock {
		return "eth_sendEndOfBlockBundle", basePayload(bundle), nil
	}
	return "eth_sendBundle", basePayload(bundle), nil
}
