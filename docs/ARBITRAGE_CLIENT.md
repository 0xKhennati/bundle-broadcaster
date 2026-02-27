# Bundle Broadcaster - Arbitrage Client Integration Guide

This document describes how to build a **Go** client that sends MEV bundles to the **bundle-broadcaster** service. Use this guide when integrating an arbitrage service or building a Cursor agent that submits bundles.

---

## Overview

The bundle-broadcaster receives JSON messages over **WebSocket**, then broadcasts them to multiple MEV relays in parallel. Your arbitrage client connects to the broadcaster and sends bundle payloads; the broadcaster handles signing, strategy selection, and multi-relay delivery.

---

## Connection

| Property | Value |
|----------|-------|
| **Endpoint** | `ws://<host>:<port>/ws` |
| **Protocol** | WebSocket |
| **Authentication** | None (WebSocket is unauthenticated) |
| **Content-Type** | JSON messages |

**Example URLs:**
```
ws://localhost:8585/ws
ws://192.168.1.10:1234/ws
wss://your-broadcaster.example.com/ws   (if TLS enabled)
```

The server address and port are configured in `config.json` under `server.address` and `server.port`.

---

## Message Format

Send a **single JSON object** per WebSocket message. All fields use the exact names below.

### Request Schema

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

### Field Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `bundle_id` | string | Yes | Unique identifier for this bundle (for logging/tracking) |
| `strategy_type` | string | Yes | One of: `target_block`, `target_tx`, `pending_block` |
| `target_block` | number | Yes | Target block number (hex string `"0x..."` also accepted) |
| `target_tx_hash` | string | No | For `target_tx` strategy: hash of the transaction to land after |
| `raw_txs` | string[] | Yes | Array of signed RLP-encoded transaction hex strings (`0x` prefix) |
| `min_timestamp` | number | No | Minimum block timestamp (seconds since epoch) |
| `max_timestamp` | number | No | Maximum block timestamp (seconds since epoch) |
| `reverting_tx_hashes` | string[] | No | Tx hashes that would cause the bundle to revert if included |

---

## Strategy Types

| Value | Behavior |
|-------|----------|
| `target_block` | Bundle targets a specific block number |
| `target_tx` | Bundle should land after `target_tx_hash` |
| `pending_block` | Bundle targets the next/pending block |

---

## Example Payloads

### Target Block (next block)

```json
{
  "bundle_id": "arb-001-0xabc123",
  "strategy_type": "target_block",
  "target_block": 21000000,
  "target_tx_hash": "",
  "raw_txs": [
    "0x02f8...",
    "0x02f8..."
  ],
  "min_timestamp": 0,
  "max_timestamp": 0,
  "reverting_tx_hashes": []
}
```

### Target Transaction (land after specific tx)

```json
{
  "bundle_id": "arb-002-0xdef456",
  "strategy_type": "target_tx",
  "target_block": 21000000,
  "target_tx_hash": "0x1234567890abcdef...",
  "raw_txs": ["0x02f8..."],
  "min_timestamp": 0,
  "max_timestamp": 0,
  "reverting_tx_hashes": []
}
```

### Pending Block (next block, time-constrained)

```json
{
  "bundle_id": "arb-003",
  "strategy_type": "pending_block",
  "target_block": 21000001,
  "target_tx_hash": "",
  "raw_txs": ["0x02f8..."],
  "min_timestamp": 1730000000,
  "max_timestamp": 1730000060,
  "reverting_tx_hashes": ["0xabcdef..."]
}
```

---

## Client Implementation Checklist

1. **Connect** to `ws://<host>:<port>/ws`
2. **Send** one JSON object per bundle (no array wrapper)
3. **Ensure** `bundle_id` is unique per bundle
4. **Ensure** `raw_txs` contains valid signed RLP hex strings
5. **Set** `target_block` to the intended block (even for `pending_block`)
6. **Do not** expect a response; the broadcaster is fire-and-forget
7. **Handle** reconnection if the WebSocket drops

---

## Go Client Example

```go
package main

import (
    "encoding/json"
    "log"

    "github.com/gorilla/websocket"
)

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

func main() {
    conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:8585/ws", nil)
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    req := BundleRequest{
        BundleID:          "bundle-001",
        StrategyType:      "target_block",
        TargetBlock:       21000000,
        RawTxs:            []string{"0x02f8..."},
        MinTimestamp:      0,
        MaxTimestamp:      0,
        RevertingTxHashes: nil,
    }
    msg, _ := json.Marshal(req)
    if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
        log.Fatal(err)
    }
}
```

**Dependency:** `go get github.com/gorilla/websocket`

---

## Endpoints Summary

| Path | Protocol | Auth | Purpose |
|------|----------|------|---------|
| `/ws` | WebSocket | None | Submit bundles |
| `/metrics` | HTTP | None | Prometheus metrics (for scrapers) |
| `/metrics/view` | HTTP | Password | HTML metrics dashboard |
| `/health` | HTTP | None | Health check |

---

## Relays

The broadcaster sends each bundle to all relays in its config. Relay URLs and strategy types are configured server-side. Your client does not need to specify relays; it only sends the bundle payload.

---

## Errors & Edge Cases

- **Invalid JSON** – Message is logged and dropped; no error is sent back to the client
- **Missing required fields** – May cause strategy build failure; bundle is dropped for that relay
- **Queue full** – If the broadcaster is overloaded, the bundle may be dropped (check metrics)
- **Connection close** – Reconnect and resend; the broadcaster does not persist bundles

---

## Environment (Server-Side)

The broadcaster requires `BROADCASTER_PRIVATE_KEY` (env) for signing relay requests. Config path can be overridden with `CONFIG_PATH`.
