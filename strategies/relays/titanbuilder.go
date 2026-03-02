package relays

import (
	"fmt"

	"github.com/0xKhennati/bundle-broadcaster/strategies"
)

// ok ok ok
type TitanbuilderBuilder struct{}

func (b *TitanbuilderBuilder) BuildRequest(bundle *strategies.IncomingBundle) (string, interface{}, error) {
	// if bundle.StrategyType == strategies.StrategyTargetBlock || bundle.StrategyType == strategies.StrategyPendingBlock {
	// 	return "eth_sendEndOfBlockBundle", map[string]interface{}{
	// 		"txs":         bundle.RawTxs,
	// 		"blockNumber": fmt.Sprintf("0x%x", bundle.TargetBlock),
	// 		"targetPools": bundle.TargetPools,
	// 	}, nil
	// }
	return "eth_sendBundle", map[string]interface{}{
		"txs":         bundle.RawTxs,
		"blockNumber": fmt.Sprintf("0x%x", bundle.TargetBlock),
	}, nil
}

// {
// 	"jsonrpc": "2.0",
// 	"id": 1,
// 	"method": "eth_sendEndOfBlockBundle",
// 	"params": [
// 	  {
// 		txs,                   // Array[String], A list of signed transactions to execute in an atomic bundle, list can be empty for bundle cancellations
// 		blockNumber,           // (Optional) String, a hex-encoded block number for which this bundle is valid. Default, current block number
// 		revertingTxHashes,     // (Optional) Array[String], A list of tx hashes that are allowed to revert or be discarded
// 		targetPools,           // Array[String], A list of pool addresses that this bundle is targeting
// 		replacementUuid,       // (Optional) String, any arbitrary string that can be used to replace or cancel this bundle
// 		replacementSeqNumber,  // (Optional) Number, monotonically increasing sequence for bundles sharing the same replacementUuid. Later bundles must have a higher sequence or they are dropped. If 0 or omitted, ordering falls back to builder receive time.
// 	  }
// 	]
//   }

// {
// 	"jsonrpc": "2.0",
// 	"id": 1,
// 	"method": "eth_sendBundle",
// 	"params": [
// 	  {
// 		txs,                   // Array[String], A list of signed transactions to execute in an atomic bundle, list can be empty for bundle cancellations
// 		blockNumber,           // (Optional) String, a hex-encoded block number for which this bundle is valid. Default, current block number
// 		revertingTxHashes,     // (Optional) Array[String], A list of tx hashes that are allowed to revert or be discarded
// 		droppingTxHashes,      // (Optional) Array[String], A list of tx hashes that are allowed to be discarded, but may not revert
// 		replacementUuid,       // (Optional) String, any arbitrary string that can be used to replace or cancel this bundle
// 		refundPercent,         // (Optional) Number, the percentage (from 0 to 99) of the  ETH reward of the last transaction, or the transaction specified by refundIndex, that should be refunded back to the ‘refundRecipient’
// 		refundRecipient,       // (Optional) Address, the address that will receive the ETH refund. Default, sender of the first transaction in the bundle
// 		replacementSeqNumber,  // (Optional) Number, monotonically increasing sequence for bundles sharing the same replacementUuid. Later bundles must have a higher sequence or they are dropped. If 0 or omitted, ordering falls back to builder receive time
// 		minTimestamp,          // (Optional) Number, the minimum slot timestamp for which this bundle is valid, in seconds since the unix epoch
// 	  }
// 	]
//   }
