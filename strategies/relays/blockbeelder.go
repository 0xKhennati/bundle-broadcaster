package relays

import (
	"fmt"

	"github.com/0xKhennati/bundle-broadcaster/strategies"
)

// https://blockbeelder.com/rpc
type BlockbeelderBuilder struct{}

func (b *BlockbeelderBuilder) BuildRequest(bundle *strategies.IncomingBundle) (string, interface{}, error) {
	params := map[string]interface{}{
		"txs":         bundle.RawTxs,
		"blockNumber": fmt.Sprintf("0x%x", bundle.TargetBlock),
	}
	if len(bundle.RevertingTxHashes) > 0 {
		params["revertingTxHashes"] = bundle.RevertingTxHashes
	}
	return "eth_sendBundle", params, nil
}
