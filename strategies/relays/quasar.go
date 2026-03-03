package relays

import (
	"fmt"

	"github.com/0xKhennati/bundle-broadcaster/strategies"
)

type QuasarBuilder struct{}

func (b *QuasarBuilder) BuildRequest(bundle *strategies.IncomingBundle) (string, interface{}, error) {
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
	return "eth_sendBundle", params, nil
}

// {
// 	"jsonrpc": "2.0",
// 	"id": 1,
// 	"method": "eth_sendBundle",
// 	"params": [
// 	  {
// 		txs,               // Array[String], A list of signed transactions to execute in an atomic bundle
// 		blockNumber,       // (Optional) String, a hex-encoded block number for which this bundle is valid. Default, current block number
// 		minTimestamp,      // (Optional) Number, the minimum timestamp for which this bundle is valid, in seconds since the unix epoch
// 		maxTimestamp,      // (Optional) Number, the maximum timestamp for which this bundle is valid, in seconds since the unix epoch
// 		revertingTxHashes, // (Optional) Array[String], A list of tx hashes that are allowed to revert
// 		droppingTxHashes,  // (Optional) Array[String], A list of tx hashes that are allowed to be discarded, but may not revert
// 		replacementUuid,   // (Optional) String, UUID that can be used to cancel/replace this bundle
// 		refundPercent,     // (Optional) Number, The percent(from 0 to 99) of full bundle ETH reward that should be passed back to the user(refundRecipient) at the end of the bundle.
// 		refundRecipient,   // (Optional) String, Address of the wallet that will receive the ETH reward refund from this bundle, default value = EOA of the first transaction inside the bundle.
// 		refundTxHashes     // (Optional) Array[String], Maximum length of 1 tx hash from which the refund is calculated. Defaults to final transaction in the bundle if list is not specified/empty.
// 	  }
// 	]
//   }
