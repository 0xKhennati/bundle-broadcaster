# Bundle Broadcaster - Client Quick Reference

Use the `client` package in your arbitrage bot to send bundles to the broadcaster.

For full documentation, see [CLIENT_USAGE.md](CLIENT_USAGE.md).

## Install

```bash
go get github.com/bundle-broadcaster/client
```

## Usage

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

## Client API

- `client.New(wsURL string) (*Client, error)` – create client, connect immediately, store URL
- `c.Send(b *client.BundleRequest) error` – send bundle (reconnects automatically if connection closed)
- `c.Close() error` – close connection (prevents reconnection)

## BundleRequest Type

Use `client.BundleRequest` and `client.StrategyTargetBlock`, `client.StrategyTargetTx`, `client.StrategyPendingBlock`:

```go
&client.BundleRequest{
    BundleID:          "bundle-001",
    StrategyType:      client.StrategyTargetBlock,
    TargetBlock:       21000000,
    TargetTxHash:      "",
    RawTxs:            []string{"0x02f8..."},
    MinTimestamp:      0,
    MaxTimestamp:      0,
    RevertingTxHashes: nil,
}
```

## Strategy Constants

- `client.StrategyTargetBlock`
- `client.StrategyTargetTx`
- `client.StrategyPendingBlock`
