# Bundle Broadcaster - Client Quick Reference

For Cursor agents building Go arbitrage clients. Copy this into your arbitrage project.

## WebSocket

- **URL:** `ws://<host>:<port>/ws`
- **Auth:** None
- **Format:** JSON, one object per message

## Request Schema (exact field names)

```json
{
  "bundle_id": "string",
  "strategy_type": "target_block | target_tx | pending_block",
  "target_block": 12345678,
  "target_tx_hash": "0x...",
  "raw_txs": ["0x...", "0x..."],
  "min_timestamp": 0,
  "max_timestamp": 0,
  "reverting_tx_hashes": []
}
```

## Required Fields

- `bundle_id` (string)
- `strategy_type` (one of: `target_block`, `target_tx`, `pending_block`)
- `target_block` (uint64)
- `raw_txs` (array of hex strings, 0x-prefixed signed RLP)

## Optional Fields

- `target_tx_hash` – for `target_tx` strategy
- `min_timestamp`, `max_timestamp` – block timestamp bounds
- `reverting_tx_hashes` – tx hashes that would revert the bundle

## Go Struct

```go
type BundleRequest struct {
    BundleID          string   `json:"bundle_id"`
    StrategyType      string   `json:"strategy_type"`
    TargetBlock       uint64   `json:"target_block"`
    TargetTxHash      string   `json:"target_tx_hash"`
    RawTxs            []string `json:"raw_txs"`
    MinTimestamp      uint64   `json:"min_timestamp"`
    MaxTimestamp      uint64   `json:"max_timestamp"`
    RevertingTxHashes []string `json:"reverting_tx_hashes"`
}
```

## Send Example (Go)

```go
conn, _, _ := websocket.DefaultDialer.Dial("ws://localhost:8585/ws", nil)
defer conn.Close()

req := BundleRequest{
    BundleID:     "bundle-001",
    StrategyType: "target_block",
    TargetBlock:  21000000,
    RawTxs:       []string{"0x02f8..."},
}
msg, _ := json.Marshal(req)
conn.WriteMessage(websocket.TextMessage, msg)
```

**Dependency:** `go get github.com/gorilla/websocket`
