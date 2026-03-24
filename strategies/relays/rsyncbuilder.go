package relays

import (
	"fmt"

	"github.com/0xKhennati/bundle-broadcaster/strategies"
)

// https://rsync-builder.xyz
// RPC: https://rsync-builder.xyz
type RsyncbuilderBuilder struct{}

func (b *RsyncbuilderBuilder) BuildRequest(bundle *strategies.IncomingBundle) (string, interface{}, error) {
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
	if bundle.RefundPercent != nil {
		params["refundPercent"] = *bundle.RefundPercent
	}
	if bundle.RefundRecipient != "" {
		params["refundRecipient"] = bundle.RefundRecipient
	}
	if len(bundle.RefundTxHashes) > 0 {
		params["refundTxHashes"] = bundle.RefundTxHashes
	}
	return "eth_sendBundle", params, nil
}
