package strategies

type RelayStrategy interface {
	BuildRequest(bundle *IncomingBundle) (method string, payload interface{}, err error)
}

const (
	StrategyTargetBlock  = "target_block"
	StrategyTargetTx     = "target_tx"
	StrategyPendingBlock = "pending_block"
)

type IncomingBundle struct {
	BundleID          string   `json:"bundle_id"`
	StrategyType      string   `json:"strategy_type"`
	TargetBlock       uint64   `json:"target_block"`
	TargetTxHash      string   `json:"target_tx_hash"`
	RawTxs            []string `json:"raw_txs"`
	MinTimestamp      uint64   `json:"min_timestamp"`
	MaxTimestamp      uint64   `json:"max_timestamp"`
	RevertingTxHashes []string `json:"reverting_tx_hashes"`
	TargetPools       []string `json:"target_pools"`

	// Refund fields — forwarded to relays that support them.
	// RefundPercent is the percentage (0–99) of the bundle's ETH reward
	// that will be sent back to RefundRecipient.
	RefundPercent    *int     `json:"refund_percent,omitempty"`
	RefundRecipient  string   `json:"refund_recipient,omitempty"`
	RefundTxHashes   []string `json:"refund_tx_hashes,omitempty"`
	// DelayedRefund and RefundIdentity are BuilderNet-specific.
	DelayedRefund  bool   `json:"delayed_refund,omitempty"`
	RefundIdentity string `json:"refund_identity,omitempty"`
}
