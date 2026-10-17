# Integration Tests

This suite proves the product works end to end:

```
mock Soroban RPC (one real event) -> indexer (-mode once) -> Postgres -> API
```

Every assertion is exact. If indexing stops writing rows, the indexing cursor
stops advancing, or the API stops serialising a field, the suite fails — it
never accepts a bare "2xx or 4xx".

## Requirements

- Docker with either the Compose v2 plugin (`docker compose`) or the
  standalone `docker-compose` binary.
- Go 1.26 (for `go test`).

## Run

```sh
cd tests/integration
make integration-test
```

`make integration-test` builds the API, indexer and mock RPC images, starts
Postgres, Redis, the mock RPC, migrations and the API, then runs the Go suite.
The suite registers a contract through `POST /api/v1/contracts`, activates it,
runs the indexer once using `docker compose run --rm indexer -mode once`, and
verifies the event, invocation, stats, filters and idempotency through the API.
The stack is torn down (including volumes) afterwards, on success or failure.

The suite can also be run against an already-running stack:

```sh
make up
go test ./... -v -count=1
make down
```

## Fixture

The mock RPC (`mock_rpc/main.go`) serves one event for
`CBIELTK6YBZJU5UP2WWQEUCYKLPU6AUNZ2BQ4WWFEIE3USCIHMXQDAMA` on testnet at ledger
999999 with a real `Symbol("transfer")` topic and `I128(1000)` value. It only
returns the event when the indexer requests the matching contract, event type
and ledger window.

Registration leaves a contract in `pending`, and the pending ->
backfilling -> active lifecycle is tracked by issue #509. Until that lands, the
suite activates its fixture row directly in Postgres so the indexer (which only
processes active/backfilling contracts) picks it up.

## CI

`.github/workflows/integration.yml` runs this suite on every pull request to
`main` and nightly on a schedule.
