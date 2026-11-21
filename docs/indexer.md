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

## Failure classification

The indexer classifies every failed invocation into one category and keeps the
raw error code it derived the category from (issue #806). The classification is
decoded from the transaction result XDR and the host diagnostic events with
`services/indexer/internal/failure` and `internal/scval`, and stored in
`invocations.failure_category` and `invocations.failure_code`. Successful
invocations leave both columns `NULL`.

| Category      | Meaning                                                                 |
| ------------- | ----------------------------------------------------------------------- |
| `budget`      | A resource budget or refundable-fee limit was hit (CPU, memory, or the declared resource fee). |
| `contract`    | The contract raised its own error with `panic`/`revert`; the code is the contract-defined number. |
| `auth`        | A contract or transaction authentication failure (`SCE_AUTH`, `tx_bad_auth`, `tx_bad_auth_extra`). |
| `storage`     | A storage error, or an archived ledger entry that must be restored first. |
| `wasm_trap`   | The Wasm VM trapped while executing contract code.                       |
| `value`       | Host value conversion or validation failed.                              |
| `crypto`      | A host cryptographic operation failed.                                   |
| `events`      | Emitting a contract event failed.                                        |
| `object`      | A failure in the host object subsystem.                                  |
| `context`     | A host context error (frame, re-entry, or state misuse).                 |
| `transaction` | The transaction itself was rejected before or around execution: bad sequence number, insufficient fee, too early/late, and so on. |
| `other`       | Everything the decoder could not attribute, including malformed host function results. |

How to read the stored values:

- When the host raised an `SCError`, the category comes from its type and the
  code is `<type>:<code>`, for example `sce_storage:3` or `sce_contract:5`.
  Contract errors carry the contract-defined number; every other type carries
  an `SCErrorCode`.
- When no host error was recorded, the category comes from the
  `InvokeHostFunctionResultCode` (for example `invoke_host_function_trapped`)
  or the `TransactionResultCode` (for example `tx_bad_seq`), and the raw code is
  that symbolic name.
- An undecodable failure is still stored as `other` rather than dropped; the
  untouched `invocations.result_xdr` holds the raw transaction result.
  Invocations indexed before this feature, and rows whose category is `NULL`,
  are counted as `other` by the stats endpoint so the totals reconcile.

The API exposes the classification as `failure_category` and `failure_code` on
every invocation, a `failure_category` filter on both invocation list
endpoints, and a per-category `failure_categories` map in each window of
`GET /api/v1/contracts/{id}/invocations/stats`.

The store tests run when `POSTGRES_TEST_URL` points at a migrated database:

```bash
POSTGRES_TEST_URL=postgres://sorolens:sorolens@localhost:5432/sorolens?sslmode=disable \
  go test ./services/indexer/internal/pgstore/...
```

## Redis run lock

| Variable   | Default                   | Description                                                                                                                                                                 |
| ---------- | ------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `REDIS_URL` | `redis://localhost:6379` | Redis URL for the per-contract run lock. The indexer acquires a Redis lock (`SET key <random token> NX PX <ttl>`) before indexing each contract, renews it while the run is active, and releases it with a compare-and-delete Lua script so one runner never deletes another's lock. An unset or empty `REDIS_URL` falls back to this default, so locking stays enabled. To disable locking (and fall back to the always-grant stub), pass an explicitly empty flag: `-redis-url=""`; then only one indexer should run per contract set. |

When the URL is non-empty (including the default), Redis is checked at startup and an unreachable server makes the indexer exit with a non-zero status, so two runners can never process the same ledgers. When locking is disabled with `-redis-url=""`, no Redis connection is attempted.
