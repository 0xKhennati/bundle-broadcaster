# Bundle Broadcaster Client - Usage Guide

This document describes how to use the `client` package to send MEV bundles from your arbitrage bot or other service to the bundle-broadcaster.

---

## Overview

The client connects to the bundle-broadcaster via WebSocket, sends bundle requests as JSON, and handles connection lifecycle automatically:

- **Connects on creation** – `New()` establishes the WebSocket connection immediately
- **Reconnects on failure** – If the connection closes, `Send()` reconnects and retries (up to 3 attempts)
- **Thread-safe** – Safe for concurrent use from multiple goroutines

---

## Installation

Add the client package to your project:

```bash
go get github.com/bundle-broadcaster/client
```

---

## Quick Start

```go
package main

import (
    "log"

    "github.com/bundle-broadcaster/client"
)

func main() {
    c, err := client.New("ws://localhost:8585/ws")
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()

    err = c.Send(&client.BundleRequest{
        BundleID:     "bundle-001",
        StrategyType: client.StrategyTargetBlock,
        TargetBlock:  21000000,
        RawTxs:       []string{"0x02f8..."},
    })
    if err != nil {
        log.Fatal(err)
    }
}
```

---

## API Reference

### New

```go
func New(wsURL string) (*Client, error)
```

Creates a new client and connects to the given WebSocket URL.

- **wsURL** – URL of the broadcaster (e.g. `ws://localhost:8585/ws` or `wss://broadcaster.example.com/ws`)
- **Returns** – `*Client` and `error` (nil on success)

**Example:**

```go
c, err := client.New("ws://192.168.1.10:8585/ws")
if err != nil {
    log.Fatal("failed to connect:", err)
}
defer c.Close()
```

---

### Send

```go
func (c *Client) Send(b *BundleRequest) error
```

Sends a bundle request to the broadcaster.

- **b** – The bundle to send (must not be nil)
- **Returns** – `nil` on success, or an error if all retries fail

**Connection behavior:**

- If the connection is closed, it reconnects automatically and resends the request
- Retries up to 3 times on connection failure
- Returns `client.ErrClientClosed` if `Close()` was called

---

### Close

```go
func (c *Client) Close() error
```

Closes the WebSocket connection and prevents further reconnection.

- Call `defer c.Close()` to ensure cleanup
- After `Close()`, `Send()` returns `client.ErrClientClosed`

---

## BundleRequest

`BundleRequest` is the type for all bundle payloads:

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

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `BundleID` | string | Yes | Unique identifier for this bundle |
| `StrategyType` | string | Yes | One of: `target_block`, `target_tx`, `pending_block` |
| `TargetBlock` | uint64 | Yes | Target block number |
| `TargetTxHash` | string | No | For `target_tx`: hash of the transaction to land after |
| `RawTxs` | []string | Yes | Signed RLP-encoded transaction hex strings (`0x` prefix) |
| `MinTimestamp` | uint64 | No | Minimum block timestamp (seconds) |
| `MaxTimestamp` | uint64 | No | Maximum block timestamp (seconds) |
| `RevertingTxHashes` | []string | No | Tx hashes that would revert the bundle |

---

## Strategy Types

Use these constants for `StrategyType`:

| Constant | Value | Description |
|----------|-------|-------------|
| `client.StrategyTargetBlock` | `"target_block"` | Bundle targets a specific block |
| `client.StrategyTargetTx` | `"target_tx"` | Bundle should land after `TargetTxHash` |
| `client.StrategyPendingBlock` | `"pending_block"` | Bundle targets the next/pending block |

---

## Examples

### Target Block (simple)

```go
err := c.Send(&client.BundleRequest{
    BundleID:     "arb-001",
    StrategyType: client.StrategyTargetBlock,
    TargetBlock:  21000000,
    RawTxs:       []string{"0x02f8..."},
})
```

### Target Transaction (land after specific tx)

```go
err := c.Send(&client.BundleRequest{
    BundleID:     "arb-002",
    StrategyType: client.StrategyTargetTx,
    TargetBlock:  21000000,
    TargetTxHash: "0x1234567890abcdef...",
    RawTxs:       []string{"0x02f8..."},
})
```

### Pending Block (with timestamp bounds)

```go
err := c.Send(&client.BundleRequest{
    BundleID:          "arb-003",
    StrategyType:      client.StrategyPendingBlock,
    TargetBlock:       21000001,
    RawTxs:            []string{"0x02f8..."},
    MinTimestamp:      1730000000,
    MaxTimestamp:      1730000060,
    RevertingTxHashes: []string{"0xabcdef..."},
})
```

### Sending multiple bundles

```go
c, err := client.New("ws://localhost:8585/ws")
if err != nil {
    log.Fatal(err)
}
defer c.Close()

for _, bundle := range bundles {
    if err := c.Send(&bundle); err != nil {
        log.Printf("send failed: %v", err)
    }
}
```

---

## Error Handling

```go
err := c.Send(&client.BundleRequest{...})
if err != nil {
    if errors.Is(err, client.ErrClientClosed) {
        log.Println("client was closed")
        return
    }
    log.Printf("send failed after retries: %v", err)
}
```

---

## Connection Behavior

1. **On `New()`** – Connects and stores the URL.

2. **On `Send()`** – Uses the existing connection. If the connection is closed:
   - Clears the connection
   - Reconnects
   - Resends the request
   - Retries up to 3 times on failure

3. **On `Close()`** – Closes the connection and sets a flag so no further reconnects occur.

4. **Thread safety** – `Send()` and `Close()` are safe for concurrent use.

---

## Best Practices

1. **Always call `Close()`** – Use `defer c.Close()` to ensure cleanup.

2. **Reuse the client** – Create one client and reuse it for multiple bundles instead of creating a new client per bundle.

3. **Handle errors** – Check `Send()` errors; retries may still fail if the broadcaster is down.

4. **Unique `BundleID`** – Use a unique ID per bundle for logging and tracking.

```go
bundleID := fmt.Sprintf("arb-%d-%s", time.Now().UnixNano(), txHash[:8])
```
