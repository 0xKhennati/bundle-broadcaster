package strategies

import (
	"fmt"
)

type DefaultBundleStrategy struct{}

func NewDefaultBundleStrategy() *DefaultBundleStrategy {
	return &DefaultBundleStrategy{}
}

func (s *DefaultBundleStrategy) BuildRequest(bundle *IncomingBundle) (string, interface{}, error) {
	payload := map[string]interface{}{
		"txs":                bundle.RawTxs,
		"blockNumber":        fmt.Sprintf("0x%x", bundle.TargetBlock),
		"minTimestamp":       bundle.MinTimestamp,
		"maxTimestamp":       bundle.MaxTimestamp,
		"revertingTxHashes":  bundle.RevertingTxHashes,
	}
	return "eth_sendBundle", payload, nil
}
