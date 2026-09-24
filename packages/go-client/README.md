# go-client

Auto-generated Go client for the Sorolens API.

The client is generated from [`docs/openapi.yaml`](../../docs/openapi.yaml) with
`oapi-codegen` and committed so it can be imported directly:

```go
import github.com/sorolens/go-client/internal/client
```

## Usage

```go
c, err := client.New("https://api.sorolens.xyz", "sl_your_api_key", nil)
if err != nil {
    panic(err)
}

resp, err := c.ListContracts(context.Background(), &client.ListContractsParams{})
if err != nil {
    panic(err)
}
if resp.JSON200 != nil {
    for _, contract := range resp.JSON200.Contracts {
        fmt.Println(contract.Id, contract.Status)
    }
}
```

When no API key is supplied, the client omits the `Authorization` header
(anonymous reads are allowed by the API for public endpoints).

## Regenerating

```sh
make client-go
```

CI (`go-client-stale`) fails on any PR where the checked-in generated code
differs from the spec, so the client is always in sync with `docs/openapi.yaml`.

## Layout

- `internal/client/client.gen.go` — generated types and HTTP client (do not edit).
- `internal/client/client.go` — thin ergonomic wrapper (`client.New`, typed
  helpers) around the generated client.
- `internal/client/client_test.go` — integration tests against an
  `httptest` stub of the API.

## Testing

```sh
cd packages/go-client && go test -race ./...
```