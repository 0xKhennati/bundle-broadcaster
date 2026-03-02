package relays

import (
	"fmt"

	"github.com/0xKhennati/bundle-broadcaster/strategies"
)

type BuildernetBuilder struct{}

func (b *BuildernetBuilder) BuildRequest(bundle *strategies.IncomingBundle) (string, interface{}, error) {
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
//     "id": 1,
//     "jsonrpc": "2.0",
//     "method": "eth_sendBundle",
//     "params": [{
//         txs               // Array[String], A list of signed transactions to execute in an atomic bundle.
//         blockNumber       // String, A hex encoded block number for which this bundle is valid on.

//         // Optional fields
//         replacementUuid   // String, UUID that can be used to cancel/replace this bundle.
//         replacementNonce  // String, Used to order bundles sharing the same replacementUuid. Bundles received with a lower replacementNonce than the latest accepted one are ignored.

//         revertingTxHashes // Array[String], A list of tx hashes that are allowed to revert.
//         droppingTxHashes  // Array[String], A list of tx hashes that can be removed from the bundle if it's deemed useful (but not revert).
//         refundTxHashes // Array[String], A list of tx hashes (max 1) that should be considered for MEV refunds. If empty, defaults to the last transaction in the bundle.

//         minTimestamp      // Number, The minimum timestamp for which this bundle is valid, in seconds since the unix epoch.
//         maxTimestamp      // Number, The maximum timestamp for which this bundle is valid, in seconds since the unix epoch.

//         refundPercent     // Number (integer between 1-99), How much of the total priority fee + coinbase payment you want to be refunded for.
//         refundRecipient   // String, The address that the funds from refundPercent will be sent to. If not specified, they will be sent to the from address of the first transaction.
//         delayedRefund     // Boolean, If true, `refundPercent` refund is processed asynchronously via BuilderNet refund pipeline rather than in the same block.

//         refundIdentity   // String, Address that BuilderNet refunds should be sent to instead of the bundle signer.
//     }]
// }
