# go-client

Auto-generated Go client for the Sorolens API.

The client is generated from [`docs/openapi.yaml`](../../docs/openapi.yaml) with
`oapi-codegen` and wrapped into an ergonomic, production-grade Go SDK with
automatic retries, rate-limit backoff, and public cursor pagination helpers:

```go
import "github.com/sorolens/go-client/internal/client"
```

## Quickstart

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/sorolens/go-client/internal/client"
)

func main() {
    c, err := client.New("https://api.sorolens.xyz", "sl_your_api_key", nil)
    if err != nil {
        log.Fatalf("failed to create client: %v", err)
    }

    resp, err := c.ListContracts(context.Background(), &client.ListContractsParams{})
    if err != nil {
        log.Fatalf("failed to list contracts: %v", err)
    }
    if resp.JSON200 != nil {
        for _, contract := range resp.JSON200.Contracts {
            fmt.Println(contract.Id, contract.Status)
        }
    }
}
```

When no API key is supplied (`""`), the client omits the `Authorization` header
(anonymous reads are allowed by the API for public endpoints).

## Retries & Rate-Limit Handling

The client includes built-in, context-aware retries with exponential backoff:

- **Automatic retry on transient errors**: HTTP `429 Too Many Requests`, `502 Bad Gateway`, `503 Service Unavailable`, `504 Gateway Timeout`, and temporary network transport errors.
- **`Retry-After` header support**: Honors both delta-seconds (e.g. `Retry-After: 60`) and RFC 1123 HTTP-date values up to the configurable cap.
- **Context-aware backoff**: Respects request deadlines and cancellations immediately during wait periods without leaking timers or connections.
- **Safe body replay**: Request payloads (e.g. for `POST` / `PUT`) are buffered and rewound on retry.

### Defaults

- Maximum retries: `3` (4 total attempts)
- Initial backoff: `100ms`
- Maximum backoff: `10s`
- Backoff multiplier: `2.0`
- `Retry-After` cap: `10s`

### Customizing Retry Behavior

Configure retries when constructing the client using functional options:

```go
// Tune retry count and backoff windows
c, err := client.New("https://api.sorolens.xyz", "sl_api_key", nil,
    client.WithMaxRetries(5),
    client.WithBackoff(50*time.Millisecond, 5*time.Second),
)

// Attach an observability callback
c, err := client.New("https://api.sorolens.xyz", "sl_api_key", nil,
    client.WithOnRetry(func(attempt int, req *http.Request, resp *http.Response, err error, wait time.Duration) {
        log.Printf("retrying %s %s (attempt %d, waiting %v): status=%v err=%v",
            req.Method, req.URL.Path, attempt, wait, resp.StatusCode, err)
    }),
)

// Provide a full custom RetryConfig
c, err := client.New("https://api.sorolens.xyz", "sl_api_key", nil,
    client.WithRetryConfig(client.RetryConfig{
        MaxRetries:            4,
        InitialBackoff:        200 * time.Millisecond,
        MaxBackoff:            8 * time.Second,
        BackoffFactor:         2.0,
        RetryAfterCap:         15 * time.Second,
        RetryableStatusCodes:  []int{http.StatusTooManyRequests, http.StatusServiceUnavailable},
        RetryableNetworkErrors: true,
    }),
)

// Disable retries entirely
c, err := client.New("https://api.sorolens.xyz", "sl_api_key", nil,
    client.WithoutRetries(),
)
```

## Pagination Helpers

The client provides type-safe `PageIterator[T]` helpers for all cursor-based endpoints.

Supported iterator methods:
- `IterateContracts(ctx, params)` — tracked contracts
- `IterateContractEvents(ctx, contractID, params)` — events for a contract
- `IterateAllEvents(ctx, params)` — global event stream
- `IterateContractInvocations(ctx, contractID, params)` — contract function calls
- `IterateContractStorage(ctx, contractID, params)` — contract storage entries
- `IterateWatchdogAlerts(ctx, params)` — watchdog alerts across monitored contracts
- `IterateMonitoredContracts(ctx, params)` — watchdog monitored contracts
- `IterateContractAlerts(ctx, contractID, params)` — watchdog alerts for a specific contract

### Item-by-Item Iteration

```go
it := c.IterateContracts(ctx, &client.ListContractsParams{
    Limit: &limit,
})
for it.Next() {
    contract := it.Item()
    fmt.Printf("%s (%s)\n", contract.Id, contract.Status)
}
if err := it.Err(); err != nil {
    log.Fatalf("iteration failed: %v", err)
}
```

### Go 1.23+ Range Over Function (`iter.Seq2`)

```go
for contract, err := range c.IterateContracts(ctx, nil).All() {
    if err != nil {
        log.Fatalf("error: %v", err)
    }
    fmt.Println(contract.Id)
}
```

### Batch / Page-Level Processing

```go
it := c.IterateContracts(ctx, nil)
for it.HasMore() {
    page, err := it.NextPage()
    if err != nil {
        log.Fatalf("page fetch failed: %v", err)
    }
    fmt.Printf("fetched batch of %d contracts\n", len(page))
}
```

### Collect All Items

```go
contracts, err := c.IterateContracts(ctx, nil).Collect()
if err != nil {
    log.Fatalf("failed to collect contracts: %v", err)
}
fmt.Printf("total contracts: %d\n", len(contracts))
```

### Resuming Pagination

Pass an existing cursor into the request params:

```go
cursor := "opaque_cursor_token"
it := c.IterateContracts(ctx, &client.ListContractsParams{
    Cursor: &cursor,
})
```

## Operation Coverage

`ApiClient` provides high-level convenience methods matching the other Sorolens SDKs:

- **Contracts**: `ListContracts`, `GetContract`, `RegisterContract`, `GetContractSnapshot`, `GetContractForecast`
- **Events & Invocations**: `ListContractEvents`, `ListAllEvents`, `ListContractInvocations`, `ListContractStorage`
- **Stats**: `GetGlobalStats`, `GetContractStats`
- **Watchdog**: `GetMonitoredContract`, `ListMonitoredContracts`, `GetWatchdogStats`, `ListWatchdogAlerts`, `ListContractAlerts`, `ListContractHealthChecks`
- **Health**: `Health`, `Readyz`
- **API Keys**: `ListApiKeys`, `CreateApiKey`, `RevokeApiKey`
- **Watchlist**: `ListWatchlist`, `AddToWatchlist`, `RemoveFromWatchlist`, `GetWatchlistStatus`
- **Advanced / Raw Access**: `c.Generated()` exposes the full underlying `*ClientWithResponses` for any route.

## Examples

Four runnable examples live in [`examples/`](examples). Each one runs offline
out of the box (`httptest` servers) and against a live API when
`SOROLENS_API_URL` is set:

| Example                                                          | Shows                                                                        |
| ---------------------------------------------------------------- | ---------------------------------------------------------------------------- |
| [`examples/pagination`](examples/pagination/main.go)             | Walking every page with `IterateContracts` and the cursor helpers.           |
| [`examples/errors`](examples/errors/main.go)                     | 404 handling plus automatic retry/backoff recovery from a 429.               |
| [`examples/sse`](examples/sse/main.go)                           | Streaming `GET /api/v1/stream/events` line by line.                          |
| [`examples/webhooks`](examples/webhooks/main.go)                 | Verifying `X-Sorolens-Signature` HMAC-SHA256 webhook deliveries.            |

```sh
cd packages/go-client
go run ./examples/pagination
SOROLENS_API_URL=https://api.sorolens.xyz go run ./examples/errors
```

CI runs every example, so they are always compilable and executable.

## Regenerating

```sh
make client-go
```

The target builds `oapi-codegen` with the pinned Go 1.26 toolchain
(`GOTOOLCHAIN=go1.26.0`). `oapi-codegen` embeds a gzipped copy of the spec in
`client.gen.go`, and gzip output differs between Go releases, so regenerating
with another toolchain produces a diff that CI rejects even when the client is
otherwise in sync.

CI (`go-client-stale`) fails on any PR where the checked-in generated code
differs from the spec, so the client is always in sync with `docs/openapi.yaml`.

## Layout

- `internal/client/client.gen.go` — generated types and HTTP client (do not edit).
- `internal/client/client.go` — ergonomic `ApiClient` wrapper with extended operation coverage.
- `internal/client/retry.go` — context-aware retry transport, backoff calculation, and `Retry-After` parsing.
- `internal/client/iterator.go` — generic `PageIterator[T]` and public pagination helpers for cursor endpoints.
- `internal/client/client_test.go` — integration tests against an `httptest` stub.
- `internal/client/retry_test.go` — retry, backoff, and rate-limit handling test suite.
- `internal/client/iterator_test.go` — pagination and iterator test suite across cursor endpoints.

## Testing

```sh
cd packages/go-client && go test -race ./...
```
