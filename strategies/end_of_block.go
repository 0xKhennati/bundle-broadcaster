package strategies

import (
	"fmt"
)

type EndOfBlockStrategy struct{}

func NewEndOfBlockStrategy() *EndOfBlockStrategy {
	return &EndOfBlockStrategy{}
}

func (s *EndOfBlockStrategy) BuildRequest(bundle *IncomingBundle) (string, interface{}, error) {
	if bundle.StrategyType == StrategyTargetBlock || bundle.StrategyType == StrategyPendingBlock {
		payload := map[string]interface{}{
			"txs":               bundle.RawTxs,
			"blockNumber":       fmt.Sprintf("0x%x", bundle.TargetBlock),
			"minTimestamp":      bundle.MinTimestamp,
			"maxTimestamp":      bundle.MaxTimestamp,
			"revertingTxHashes": bundle.RevertingTxHashes,
		}
		return "eth_sendEndOfBlockBundle", payload, nil
	}
	payload := map[string]interface{}{
		"txs":               bundle.RawTxs,
		"blockNumber":       fmt.Sprintf("0x%x", bundle.TargetBlock),
		"minTimestamp":      bundle.MinTimestamp,
		"maxTimestamp":      bundle.MaxTimestamp,
		"revertingTxHashes": bundle.RevertingTxHashes,
	}
	return "eth_sendBundle", payload, nil
}
