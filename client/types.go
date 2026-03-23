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

	// Refund configuration. RefundPercent (0–99) controls how much of the
	// bundle's ETH reward is returned to RefundRecipient (defaults to the
	// sender of the first tx if omitted). RefundTxHashes pins which tx
	// the refund is calculated from (defaults to the last tx in the bundle).
	// DelayedRefund and RefundIdentity are BuilderNet-only options.
	RefundPercent  *int     `json:"refund_percent,omitempty"`
	RefundRecipient string  `json:"refund_recipient,omitempty"`
	RefundTxHashes []string `json:"refund_tx_hashes,omitempty"`
	DelayedRefund  bool     `json:"delayed_refund,omitempty"`
	RefundIdentity string   `json:"refund_identity,omitempty"`
}
