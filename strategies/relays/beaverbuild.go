package relays

import (
	"fmt"

	"github.com/0xKhennati/bundle-broadcaster/strategies"
)

type BeaverbuildBuilder struct{}

func (b *BeaverbuildBuilder) BuildRequest(bundle *strategies.IncomingBundle) (string, interface{}, error) {
	return "eth_sendBundle", map[string]interface{}{
		"txs":         bundle.RawTxs,
		"blockNumber": fmt.Sprintf("0x%x", bundle.TargetBlock),
	}, nil
}
