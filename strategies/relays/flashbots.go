package relays

import (
	"fmt"

	"github.com/0xKhennati/bundle-broadcaster/strategies"
)

type FlashbotsBuilder struct{}

func (b *FlashbotsBuilder) BuildRequest(bundle *strategies.IncomingBundle) (string, interface{}, error) {
	return "eth_sendBundle", map[string]interface{}{
		"txs":         bundle.RawTxs,
		"blockNumber": fmt.Sprintf("0x%x", bundle.TargetBlock),
	}, nil
}

func basePayload(bundle *strategies.IncomingBundle) map[string]interface{} {
	return map[string]interface{}{
		"txs":         bundle.RawTxs,
		"blockNumber": fmt.Sprintf("0x%x", bundle.TargetBlock),
		"builders": []string{"f1b.io","rsync","beaverbuild.org","builder0x69","Titan","EigenPhi","boba-builder","Gambit Labs","payload","Loki","BuildAI","JetBuilder","tbuilder","penguinbuild","bobthebuilder","BTCS","bloXroute","Blockbeelder","Quasar","Eureka",},
	}
}

// {
// 	"jsonrpc": "2.0",
// 	"id": 1,
// 	"method": "eth_sendBundle",
// 	"params": [
// 	  {
// 		txs,               // Array[String], A list of signed transactions to execute in an atomic bundle
// 		blockNumber,       // String, a hex encoded block number for which this bundle is valid on
// 		minTimestamp,      // (Optional) Number, the minimum timestamp for which this bundle is valid, in seconds since the unix epoch
// 		maxTimestamp,      // (Optional) Number, the maximum timestamp for which this bundle is valid, in seconds since the unix epoch
// 		revertingTxHashes, // (Optional) Array[String], A list of tx hashes that are allowed to revert
// 		replacementUuid,   // (Optional) String, UUID that can be used to cancel/replace this bundle
// 		builders,          // (Optional) Array[String], A list of [registered](https://github.com/flashbots/dowg/blob/main/builder-registrations.json) block builder names to share the bundle with
// 	  }
// 	]
//   }


{
	"name": "flashbots",
	"rpc": "rpc.flashbots.net",
	"supported-apis": ["v0.1"]
},

"f1b.io","rsync","beaverbuild.org","builder0x69","Titan","EigenPhi","boba-builder","Gambit Labs","payload","Loki","BuildAI","JetBuilder","tbuilder","penguinbuild","bobthebuilder","BTCS","bloXroute","Blockbeelder","Quasar","Eureka",