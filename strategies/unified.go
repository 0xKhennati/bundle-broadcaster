package strategies

import (
	"fmt"
)

type UnifiedBundleStrategy struct{}

func NewUnifiedBundleStrategy() *UnifiedBundleStrategy {
	return &UnifiedBundleStrategy{}
}

func (s *UnifiedBundleStrategy) BuildRequest(bundle *IncomingBundle) (string, interface{}, error) {
	payload := s.formatPayload(bundle)
	return "eth_sendBundle", payload, nil
}

func (s *UnifiedBundleStrategy) formatPayload(bundle *IncomingBundle) map[string]interface{} {
	return map[string]interface{}{
		"txs":               bundle.RawTxs,
		"blockNumber":       fmt.Sprintf("0x%x", bundle.TargetBlock),
		"minTimestamp":      bundle.MinTimestamp,
		"maxTimestamp":      bundle.MaxTimestamp,
		"revertingTxHashes": bundle.RevertingTxHashes,
		"targetPools":       bundle.TargetPools,
	}
}
