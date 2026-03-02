package relays

import (
	"fmt"

	"github.com/0xKhennati/bundle-broadcaster/strategies"
)

type FlashbotsBuilder struct{}

func (b *FlashbotsBuilder) BuildRequest(bundle *strategies.IncomingBundle) (string, interface{}, error) {
	return "eth_sendBundle", basePayload(bundle), nil
}

func basePayload(bundle *strategies.IncomingBundle) map[string]interface{} {
	return map[string]interface{}{
		"txs":               bundle.RawTxs,
		"blockNumber":       fmt.Sprintf("0x%x", bundle.TargetBlock),
		"minTimestamp":      bundle.MinTimestamp,
		"maxTimestamp":      bundle.MaxTimestamp,
		"revertingTxHashes": bundle.RevertingTxHashes,
		"targetPools":       bundle.TargetPools,
	}
}
