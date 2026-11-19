# Indexer configuration

The indexer (`services/indexer`) reads contract events and transactions from a
Soroban RPC node and writes them to the Postgres database the API serves. This
page lists the environment variables it needs to reach those services. Sharding
is covered separately in [`indexer-sharding.md`](indexer-sharding.md) and
Prometheus metrics in [`metrics.md`](metrics.md).

## Soroban RPC

The indexer refuses to start unless at least one RPC URL is set.

| Variable                    | Default | Description                                                                                                                                                                                          |
| --------------------------- | ------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `SOROBAN_RPC_URL`           | —       | Default endpoint. Used for every contract whose network has no dedicated URL below.                                                                                                                  |
| `SOROBAN_RPC_URL_TESTNET`   | —       | Endpoint for contracts tracked on `testnet`.                                                                                                                                                         |
| `SOROBAN_RPC_URL_MAINNET`   | —       | Endpoint for contracts tracked on `mainnet`.                                                                                                                                                         |
| `SOROBAN_RPC_URL_FUTURENET` | —       | Endpoint for contracts tracked on `futurenet`.                                                                                                                                                       |
| `SOROBAN_RPC_TIMEOUT`       | `30s`   | Timeout for a single RPC request (Go duration syntax).                                                                                                                                               |
| `SOROBAN_RPC_MAX_RETRIES`   | `3`     | Retries after HTTP 429, HTTP 5xx, or a transport failure such as a timeout. Backoff starts at 500ms, doubles each time, and is capped at 10s. A 429 `Retry-After` header is honoured up to that cap. |

JSON-RPC application errors such as an out-of-range `startLedger` are not
retried. They come back as `sorobanrpc.RPCError`, which keeps the node's error
code.

Public endpoints:

```bash
export SOROBAN_RPC_URL_TESTNET=https://soroban-testnet.stellar.org
export SOROBAN_RPC_URL_FUTURENET=https://rpc-futurenet.stellar.org
# Mainnet has no SDF-hosted public RPC; use a provider such as
# https://developers.stellar.org/docs/data/apis/rpc/providers
```

Soroban RPC keeps 7 days of history by default, so a newly tracked contract can
only backfill that far. See [New-contract backfill](#new-contract-backfill).

## New-contract backfill

A contract moves through three states, stored per contract:

| Status        | Meaning                                                                    |
| ------------- | -------------------------------------------------------------------------- |
| `pending`     | Registered; the indexer has not picked it up yet.                          |
| `backfilling` | New events are tailed while older ledgers are scanned from newest to oldest. |
| `active`      | The backfill finished and `backfill_complete_at` is set.                   |

The backfill reaches 100,000 ledgers (about 5.8 days) behind the chain head, which
fits inside the 7-day default retention. If a node retains less, the indexer
detects the out-of-range error and shortens the backfill to what the node still
serves.

Each window's rows and the new position commit in one transaction to the
`contract_backfills` table, so killing the indexer at any point resumes at the
first unscanned ledger. One contract spends at most 30 seconds per pass on its
backfill, then continues on the next pass.

| Variable                  | Default | Description                                                                                                                                                 |
| ------------------------- | ------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `INDEXER_BACKFILL_WINDOW` | `10000` | Ledgers scanned per backfill step (`--backfill-window`). Capped at `--ledger-window`. Lower it if a busy contract fails with `more than 100 pages`. |

`GET /api/v1/contracts/{id}` and `GET /api/v2/contracts/{id}` report progress in
the `backfill` object:

```json
{
  "status": "backfilling",
  "backfill": {
    "oldest_ledger": 900000,
    "newest_ledger": 1000000,
    "next_ledger": 950000,
    "ledgers_total": 100001,
    "ledgers_done": 50000,
    "percent": 49.99,
    "started_at": "2026-11-19T10:00:00Z",
    "updated_at": "2026-11-19T10:04:00Z",
    "completed_at": null
  }
}
```

History older than the RPC window comes from the optional
[Hubble importer](hubble-import.md).

## Postgres

| Variable       | Default | Description                                                                                                                                                                      |
| -------------- | ------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `DATABASE_URL` | —       | Required. pgx connection string for the database the API reads, migrated with `apps/api/internal/db/migrations`. The indexer pings it on startup and exits if it is unreachable. |

The indexer writes events, invocations, per-contract sync state, network
cursors, Wasm upgrades, the Wasm cache, health scores, and anomaly alerts
through `services/indexer/internal/pgstore`. Every batch insert ignores rows
that already exist, so re-indexing a ledger window is safe.

The store tests run when `POSTGRES_TEST_URL` points at a migrated database:

```bash
POSTGRES_TEST_URL=postgres://sorolens:sorolens@localhost:5432/sorolens?sslmode=disable \
  go test ./services/indexer/internal/pgstore/...
```
