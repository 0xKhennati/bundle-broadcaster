package client

const (
	StrategyTargetBlock  = "target_block"
	StrategyTargetTx     = "target_tx"
	StrategyPendingBlock = "pending_block"
)

// BundleRequest is the message format sent to the bundle-broadcaster WebSocket.
// Use this type when sending bundles from an arbitrage bot or other service.
type BundleRequest struct {
	BundleID          string   `json:"bundle_id"`
	StrategyType      string   `json:"strategy_type"`
	TargetBlock       uint64   `json:"target_block"`
	TargetTxHash      string   `json:"target_tx_hash"`
	RawTxs            []string `json:"raw_txs"`
	MinTimestamp      uint64   `json:"min_timestamp"`
	MaxTimestamp      uint64   `json:"max_timestamp"`
	RevertingTxHashes []string `json:"reverting_tx_hashes"`
	TargetPools       []string `json:"target_pools"`
}
