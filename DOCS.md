# bundle-broadcaster — Complete Documentation

## Table of Contents

1. [What this project does](#1-what-this-project-does)
2. [How a bundle travels through the system](#2-how-a-bundle-travels-through-the-system)
3. [File map](#3-file-map)
4. [Supported relay builders](#4-supported-relay-builders)
5. [Configuration reference](#5-configuration-reference)
6. [Features](#6-features)
   - [Simulation (eth_callBundle)](#61-simulation-eth_callbundle)
   - [Bundle tracking](#62-bundle-tracking)
   - [Refund defaults](#63-refund-defaults)
   - [Rate-limit protection](#64-rate-limit-protection)
   - [Prometheus metrics](#65-prometheus-metrics)
7. [WebSocket API (bot → broadcaster)](#7-websocket-api-bot--broadcaster)
8. [bundle_id explained](#8-bundle_id-explained)
9. [How to add a new relay](#9-how-to-add-a-new-relay)
10. [Key MEV concepts](#10-key-mev-concepts)
11. [Debugging bundles that don't land](#11-debugging-bundles-that-dont-land)
12. [Common errors and fixes](#12-common-errors-and-fixes)

---

## 1. What this project does

`bundle-broadcaster` is a Go server that sits between your arbitrage bot and multiple Ethereum block builders.

```
Arbitrage Bot  ──WebSocket──▶  bundle-broadcaster  ──HTTP (parallel)──▶  Titanbuilder
                                                                      ──▶  BuilderNet
                                                                      ──▶  BeaverBuild
                                                                      ──▶  Flashbots
                                                                      ──▶  ... (all builders)
```

Your bot sends **one JSON message** over WebSocket. The broadcaster fans it out to **all configured builders simultaneously** in parallel goroutines. Every builder receives the bundle at nearly the same time — the slowest builder does not delay the others.

---

## 2. How a bundle travels through the system

```
ws_server.go          — receives JSON from bot, deserialises into IncomingBundle,
                        pushes into a worker queue (capacity 64 workers)
        │
        ▼
relay_manager.go      — applyRefundDefaults() fills any missing refund fields
                        from config, then:
                        1. fires SimulateAsync() in background goroutine (if enabled)
                        2. fans out to all RelayClients in parallel (sync.WaitGroup)
        │
        ├─▶ simulator.go   — eth_callBundle → Flashbots (background, never blocks)
        │                    logs result, generates Tenderly links if 0 payment,
        │                    sends SimEvent to Tracker
        │
        └─▶ relay_client.go (one per builder)
               — checks 429 cooldown, skips if still cooling
               — calls builder.BuildRequest() to get relay-specific JSON
               — signs with X-Flashbots-Signature header
               — retries up to 3 times on connection errors
               — on 2xx: records Prometheus counter, sends HashEvent to Tracker
               — on 429: sets cooldown timer, records Prometheus counter
        │
        ▼
tracker.go            — single goroutine owns all file I/O
                        correlates HashEvents + SimEvents by bundle_id
                        flushes BundleRecord to tracking/{builder}.jsonl after 2s
```

---

## 3. File map

| File | Responsibility |
|---|---|
| `main.go` | Entry point, HTTP server, graceful shutdown |
| `config.go` | Config structs, JSON loading, URL helpers |
| `config.json` | Runtime configuration (edit this) |
| `ws_server.go` | WebSocket listener, worker pool (64 workers) |
| `relay_manager.go` | Orchestrates all relay clients, applies refund defaults |
| `relay_client.go` | HTTP transport for one relay (signing, retries, 429 handling) |
| `simulator.go` | `eth_callBundle` background simulation, Tenderly URL builder |
| `tracker.go` | JSONL tracking files per builder |
| `metrics.go` | Prometheus metric definitions |
| `signer.go` | ECDSA signing for `X-Flashbots-Signature` |
| `strategies/types.go` | `IncomingBundle` struct (shared with relay packages) |
| `strategies/relays/*.go` | One file per builder — implements `BuildRequest()` |
| `strategies/relays/register.go` | Registers all builders at startup via `init()` |

---

## 4. Supported relay builders

| Config name | URL | Refund support | Notes |
|---|---|---|---|
| `titanbuilder` | `eu.rpc.titanbuilder.xyz` | `refundPercent`, `refundRecipient` | EU endpoint used |
| `buildernet` | `direct-eu.buildernet.org` | `refundPercent`, `refundRecipient`, `refundTxHashes`, `delayedRefund`, `refundIdentity` | Most refund options |
| `beaverbuild` | `rpc.beaverbuild.org` | No | Standard `eth_sendBundle` only |
| `flashbots` | `relay.flashbots.net` | No | Also used for `eth_callBundle` simulation |
| `quasar` | `rpc.quasar.win` | `refundPercent`, `refundRecipient` | Has `quasar_getBundleStats` endpoint for post-hoc debugging |
| `bobthebuilder` | `rpc.bobthebuilder.xyz` | No | |
| `eurekabuilder` | `rpc.eurekabuilder.xyz` | No | |
| `jetbldr` | `rpc.jetbldr.xyz` | `refundPercent`, `refundRecipient`, `refundTxHashes` | |
| `rsyncbuilder` | `rsync-builder.xyz` | `refundPercent`, `refundRecipient`, `refundTxHashes` | DNS issues observed — remove if failing |
| `tbuilder` | `rpc.tbuilder.xyz` | `refundPercent`, `refundRecipient`, `refundTxHashes` | Timeout issues observed — remove if failing |
| `turbobuilder` | `rpc.turbobuilder.xyz` | No | DNS issues observed — remove if failing |
| `snailbuilder` | `rpc.snailbuilder.xyz` | No | Returns `null` result — likely unsupported |

> **Note:** `rsyncbuilder`, `tbuilder`, `turbobuilder`, and `snailbuilder` are registered in code but have been removed from `config.json` due to observed network failures. Add them back by inserting an entry in the `relays` array if their endpoints become stable.

---

## 5. Configuration reference

All settings live in `config.json` (or the path set in `CONFIG_PATH` env var).

```jsonc
{
  "server": {
    "address": "0.0.0.0",  // bind address
    "port": 8585            // WebSocket + HTTP port
  },

  "auth": {
    "password_hash": "<md5>",  // MD5 hash of the WebSocket password
    "max_attempts": 5,         // lock out after N bad passwords
    "lockout_minutes": 15
  },

  "private_key": "<hex>",  // 32-byte hex key for X-Flashbots-Signature
                           // OR set env var BROADCASTER_PRIVATE_KEY

  "log_level": "info",     // debug | info | warn | error | trace

  "relays": [
    {
      "name": "titanbuilder",            // must match a registered builder name
      "url": "https://eu.rpc.titanbuilder.xyz/",
      "warmup_connections": 10,          // parallel TCP connections pre-opened at startup
                                         //   0 = use default (100)
                                         //  -1 = skip warmup for this relay
      "rate_limit_cooldown_ms": 1000     // ms to pause after a 429 response
                                         // default: 1000ms
    }
  ],

  "refund": {
    // Broadcaster-level defaults applied to every bundle.
    // Values set by the bot in the WebSocket message always take priority.
    "percent": 90,                           // 0–99, percentage of reward refunded
    "recipient": "0xYourWallet",             // address that receives the refund
    "tx_hashes": [],                         // which txs the refund is calculated from
                                             // (defaults to last tx in bundle)
    "delayed_refund": false,                 // BuilderNet only: async refund pipeline
    "refund_identity": ""                    // BuilderNet only: override refund recipient
  },

  "simulate": {
    "enabled": false,                        // set true to enable background simulation
    "url": "https://relay.flashbots.net"     // endpoint for eth_callBundle
  },

  "tracking": {
    "enabled": false,                        // set true to write JSONL tracking files
    "dir": "tracking",                       // directory for output files
                                             // use absolute path on server, e.g.
                                             // "/opt/bundle-broadcaster/tracking"
    "builders": ["titanbuilder", "beaverbuild", "buildernet"]
                                             // which builders to track
  }
}
```

---

## 6. Features

### 6.1 Simulation (`eth_callBundle`)

**Config key:** `simulate.enabled`

When enabled, every incoming bundle is sent to `eth_callBundle` on Flashbots **before** being broadcast to builders. This runs in a background goroutine and never adds latency to the real broadcast.

**What it logs:**

| Scenario | Log message |
|---|---|
| HTTP error | `[simulate] eth_callBundle request failed` |
| Bundle rejected by Flashbots | `[simulate] bundle REJECTED by flashbots` with error code |
| A transaction reverted | `[simulate] tx REVERTED — bundle will be dropped by builders` with tx hash and revert reason |
| Valid but zero coinbase payment | `[simulate] bundle VALID but pays 0 to builder` + Tenderly links for every tx |
| Valid with nonzero payment | `[simulate] bundle VALID ✓` with `coinbase_diff_eth`, `eth_to_builder_eth`, `gas_fees_eth`, `total_gas_used` |

**Tenderly links:** When `eth_to_builder_eth=0`, the log includes one `tenderly_tx0`, `tenderly_tx1`, … field per transaction. Each is a pre-filled URL at `https://dashboard.tenderly.co/simulator/new?...` that opens the transaction in Tenderly's simulator anchored to the simulation block state. Use these to inspect your arbitrage contract and check whether `transfer(block.coinbase, profit)` is being called.

**Important:** `eth_callBundle` simulates against `stateBlockNumber: "latest"` — the current chain tip, applying all current transactions. It does **not** simulate top-of-block or after any specific pending transaction. Use it only to check:
1. Do transactions revert?
2. Does the bundle pay the builder (`eth_to_builder_eth > 0`)?

---

### 6.2 Bundle tracking

**Config key:** `tracking.enabled`

Writes one JSONL file per tracked builder. Each line is a JSON record for one bundle, combining the bundle hash returned by the builder with the simulation result.

**Output location:** `{tracking.dir}/{builder}.jsonl`
- e.g. `tracking/titanbuilder.jsonl`
- Use an absolute path in `tracking.dir` on a server to avoid working-directory ambiguity.
- To find the working directory of the running service: `systemctl show bundle-broadcaster --property=WorkingDirectory`

**Record format:**
```json
{
  "bundle_id": "0x...",          // ID sent by your bot (see section 8)
  "builder": "titanbuilder",
  "bundle_hash": "0x...",        // hash returned by builder's eth_sendBundle response
  "last_tx_hash": "0x...",       // hash of the LAST transaction (your backrun tx)
  "target_block": 24726517,
  "timestamp": "2026-03-24T10:32:49Z",
  "coinbase_diff_eth": "0.015",  // from simulation (empty if simulate disabled)
  "eth_to_builder_eth": "0.015",
  "gas_fees_eth": "0.000",
  "total_gas_used": 352566,
  "bundle_gas_price": "102537309",
  "sim_state_block": 24726516
}
```

**Correlation timing:** Simulation fires before broadcast. By the time a builder returns a bundle hash (~50–500ms), the sim result is usually already cached. The tracker goroutine waits 2 seconds before flushing a record to disk, giving stragglers time to arrive. Sim cache entries expire after 30 seconds.

**Use case:** Send `bundle_hash` + `last_tx_hash` + `coinbase_diff_eth` to a builder's support team and ask why your bundle was not included in `target_block`. The `bundle_hash` is the builder's own identifier for your submission.

---

### 6.3 Refund defaults

**Config key:** `refund`

Refunds are a builder mechanism to attract order flow. When you set `refundPercent: 90`, the builder returns 90% of your coinbase payment back to `refundRecipient`. This lets you bid aggressively (high gross payment) while keeping most of the profit, and compete with bots that bid lower absolute amounts.

**How it works:**
- You pay `X ETH` to `block.coinbase` in your arbitrage contract.
- Builder includes your bundle and earns `X ETH`.
- Builder sends `0.9 * X ETH` back to your `refundRecipient` address.
- Builder keeps `0.1 * X ETH` net — still competitive vs other bundles paying `< 0.1X`.

**Bot values take priority.** The `refund` block in config is only a fallback. If the bot sends `refund_percent` in the WebSocket message, that value is used instead.

**Per-relay support:** Refund fields are only included in requests to relays that document support for them. Flashbots, BeaverBuild, and EurekaBuilder receive standard `eth_sendBundle` without refund fields even if configured.

---

### 6.4 Rate-limit protection

**Per-relay config:** `warmup_connections`, `rate_limit_cooldown_ms`

When a relay returns HTTP 429, the relay client sets a timestamp and skips all bundles for that relay until the cooldown expires. The broadcaster continues sending to all other relays unaffected.

- Default cooldown: 1000ms (1 second)
- BuilderNet is configured with 2000ms cooldown in `config.json`
- Prometheus metric `bundle_rate_limited_total{relay="..."}` counts skipped bundles

**Connection warmup:** At startup, the broadcaster pre-opens TCP connections to every relay to avoid cold-start latency on the first bundle.
- Default: 100 parallel connections per relay
- `warmup_connections: 5` reduces load for rate-limited relays (BuilderNet)
- `warmup_connections: -1` skips warmup entirely for a relay

**Worker pool:** 64 concurrent workers process bundles from the queue. Each worker holds its slot until all relay goroutines complete (`wg.Wait()`). Slow relays do not delay other relays, but they do delay freeing the worker slot for the next bundle.

---

### 6.5 Prometheus metrics

Exposed at `GET /metrics` (Prometheus format) and `GET /metrics/view` (human-readable HTML, password-protected if `auth` is configured).

| Metric | Type | Labels | Description |
|---|---|---|---|
| `bundle_received_total` | Counter | — | Bundles received over WebSocket |
| `bundle_sent_total` | Counter | `relay` | Successful 2xx responses from a relay |
| `bundle_failed_total` | Counter | `relay` | Failed requests (network error or non-2xx/429) |
| `bundle_rate_limited_total` | Counter | `relay` | Bundles skipped because relay is in 429 cooldown |
| `relay_latency_ms` | Histogram | `relay` | HTTP round-trip time in milliseconds (buckets: 10ms–40s) |

---

## 7. WebSocket API (bot → broadcaster)

Your bot connects to `ws://{host}:{port}/ws` and sends one JSON message per bundle.

**Message format:**
```json
{
  "bundle_id": "0xdeadbeef...",      // REQUIRED for tracking — unique ID for this bundle
                                     // recommended: keccak256 of raw_txs joined
  "strategy_type": "target_block",   // "target_block" | "target_tx" | "pending_block"
  "target_block": 24726517,          // block number to include in (hex or decimal)
  "target_tx_hash": "",              // for target_tx strategy: hash of tx to backrun
  "raw_txs": ["0x...", "0x..."],     // signed transactions in order
  "min_timestamp": 0,                // optional: Unix seconds
  "max_timestamp": 0,                // optional: Unix seconds
  "reverting_tx_hashes": [],         // tx hashes allowed to revert

  // Refund overrides (optional — config defaults used if omitted)
  "refund_percent": 90,
  "refund_recipient": "0x...",
  "refund_tx_hashes": [],
  "delayed_refund": false,
  "refund_identity": ""
}
```

---

## 8. bundle_id explained

`bundle_id` is **set by your arbitrage bot** before sending the WebSocket message. The broadcaster never generates it — it only passes it through to logs, simulation, and tracking.

From the logs: `bundle_id=0xd2421e67...` — your bot is already computing a 32-byte hash.

**Why it matters:**
- All log lines for the same bundle share the same `bundle_id` — grep by it to trace one bundle end-to-end across simulation + all relays.
- The tracking JSONL uses it to correlate the simulation result with each builder's bundle hash.
- If `bundle_id` is empty (`""`), tracking records will all have the same empty ID and will be useless.

**Recommended value:** `keccak256(raw_txs[0] + raw_txs[1] + ...)` or a UUID generated per bundle submission.

---

## 9. How to add a new relay

1. **Create the builder file** in `strategies/relays/newrelay.go`:

```go
package relays

import (
    "fmt"
    "github.com/0xKhennati/bundle-broadcaster/strategies"
)

type NewRelayBuilder struct{}

func (b *NewRelayBuilder) BuildRequest(bundle *strategies.IncomingBundle) (string, interface{}, error) {
    params := map[string]interface{}{
        "txs":         bundle.RawTxs,
        "blockNumber": fmt.Sprintf("0x%x", bundle.TargetBlock),
    }
    // Add refund fields if the relay supports them:
    // if bundle.RefundPercent != nil {
    //     params["refundPercent"] = *bundle.RefundPercent
    // }
    return "eth_sendBundle", params, nil
}
```

2. **Register it** in `strategies/relays/register.go`:

```go
strategies.RegisterRelay("newrelay", &NewRelayBuilder{})
```

3. **Add it to config.json**:

```json
{ "name": "newrelay", "url": "https://rpc.newrelay.xyz" }
```

4. **Build and restart**: `go build ./... && systemctl restart bundle-broadcaster`

---

## 10. Key MEV concepts

**Block builder** — an entity that assembles the contents of an Ethereum block. Builders receive bundles from searchers and choose which to include based on how much ETH they earn. Major builders: Titanbuilder, BeaverBuild, BuilderNet, Flashbots, Quasar.

**Bundle** — an ordered, atomic group of signed transactions submitted privately to a builder. Either all transactions execute in order or the entire bundle is dropped. This prevents partial execution and front-running of your backrun.

**Coinbase payment** — ETH transferred from your arbitrage contract to `block.coinbase` (the builder's address for that block). This is how you pay the builder to include your bundle. Without this payment, builders have no incentive to include your bundle over others.

```solidity
// In your arbitrage contract — REQUIRED for bundle inclusion
uint256 profit = address(this).balance - startBalance;
uint256 bribe = (profit * BRIBE_PERCENT) / 100;
block.coinbase.transfer(bribe);
```

**`coinbase_diff_eth`** — total ETH earned by the builder from your bundle (gas fees + direct coinbase transfer). If this is `0.000000000` in simulation, builders will never include your bundle regardless of how many you send.

**`eth_to_builder_eth`** — the direct `transfer(block.coinbase, ...)` amount. Usually equals `coinbase_diff_eth` because your transactions likely have low base fee.

**Refund** — builder returns a percentage of your coinbase payment back to you. Lets you bid high (to win over competing bundles) while recouping most of the cost. Not all builders support this.

**Bundle hash** — the identifier a builder assigns to your submission after accepting it. Different from `bundle_id`. Used when contacting builder support.

**`target_tx_hash`** — instead of targeting a block number, your bundle is placed immediately after a specific pending transaction (the transaction you are backrunning). More precise than `target_block` for backrunning.

---

## 11. Debugging bundles that don't land

### Step 1 — Check simulation logs

Enable `simulate.enabled: true` and watch for:

| Log | Problem |
|---|---|
| `tx REVERTED` | A transaction in your bundle fails — fix the revert first |
| `coinbase_diff_eth=0.000000000` | Your contract does not pay `block.coinbase` — builders will not include it |
| `bundle REJECTED by flashbots` | Bundle is malformed or txs are invalid |
| `eth_callBundle request failed` | Network issue with Flashbots simulation endpoint |

### Step 2 — Open Tenderly links

When `eth_to_builder_eth=0`, the log prints a `tenderly_tx0` URL for each transaction. Open it and look at the execution trace:
- Is the profit calculation correct?
- Is `block.coinbase.transfer(amount)` being called?
- Is the amount `> 0`?

### Step 3 — Check Quasar bundle stats

Quasar has a unique `quasar_getBundleStats` endpoint. After sending a bundle, call it with the bundle hash to get:
- Whether the bundle was simulated
- Whether it was profitable
- Why it was excluded (e.g. "revert", "low value", "replaced")

### Step 4 — Check tracking records

Enable `tracking.enabled: true` and `simulate.enabled: true` together. After a few minutes:

```bash
tail -f tracking/titanbuilder.jsonl | python3 -m json.tool
```

Look for bundles where `eth_to_builder_eth` is nonzero but the bundle still did not land. Note the `bundle_hash` and `last_tx_hash`, then contact the builder with:

> "Bundle `{bundle_hash}`, last tx `{last_tx_hash}`, target block `{target_block}`, simulation showed `{coinbase_diff_eth}` ETH to builder — why was it not included?"

### Step 5 — Check if builders are receiving bundles

Check `bundle_sent_total` vs `bundle_failed_total` in Prometheus at `/metrics`. If `bundle_failed_total` is high for a specific builder, that builder is rejecting or unreachable.

---

## 12. Common errors and fixes

| Error | Cause | Fix |
|---|---|---|
| `context deadline exceeded` after 3 attempts | Builder's HTTP endpoint is slow or unreachable | Remove the builder from `config.json` |
| `dial tcp: lookup ... no such host` | DNS cannot resolve the builder domain | Check DNS on your server; remove the builder if domain is gone |
| `relay returned 429, entering cooldown` | Too many requests to that builder | Increase `rate_limit_cooldown_ms`, reduce `warmup_connections`, or reduce workers in `ws_server.go` |
| `{"result":null,"id":1}` from builder | Builder accepted the HTTP request but silently rejected the bundle | Builder likely does not support your `eth_sendBundle` format; remove it |
| `[simulate] bundle VALID but pays 0` | Arbitrage contract does not transfer profit to `block.coinbase` | Add `block.coinbase.transfer(bribe)` at the end of your arbitrage function |
| `[simulate] tx REVERTED` | A transaction in the bundle fails during simulation | Fix the revert in your contract; use the Tenderly link in the log to debug |
| `relay not registered, skipping` | A relay name in `config.json` has no corresponding builder in `strategies/relays/` | Add a builder file and register it (see section 9) |
| `queue full, dropping bundle` | 64 workers are all busy | Slow relays are holding worker slots; remove slow relays or increase workers |
