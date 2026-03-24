package relays

import (
	"fmt"

	"github.com/0xKhennati/bundle-broadcaster/strategies"
)

// https://rpc.turbobuilder.xyz
type TurbobuilderBuilder struct{}

func (b *TurbobuilderBuilder) BuildRequest(bundle *strategies.IncomingBundle) (string, interface{}, error) {
	params := map[string]interface{}{
		"txs":         bundle.RawTxs,
		"blockNumber": fmt.Sprintf("0x%x", bundle.TargetBlock),
	}
	if len(bundle.RevertingTxHashes) > 0 {
		params["revertingTxHashes"] = bundle.RevertingTxHashes
	}
	if bundle.MinTimestamp > 0 {
		params["minTimestamp"] = bundle.MinTimestamp
	}
	if bundle.MaxTimestamp > 0 {
		params["maxTimestamp"] = bundle.MaxTimestamp
	}
	return "eth_sendBundle", params, nil
}
