# Concepts

Sorolens has four moving parts. Understanding how they fit together makes the
dashboard and API easier to reason about.

## Indexer

The indexer polls Soroban RPC for new ledgers, decodes contract events and
invocations, and writes them to Postgres. It commits the ledger cursor in the
same transaction as the indexed rows, so a restart never skips or duplicates
data.

The indexer runs in two modes:

- `--mode once` performs a single cycle and exits. This is a good fit for a
  scheduled job.
- `--mode continuous` polls on an interval and stays resident.

## Data model

- `contracts` stores tracked contracts, labels, tags and sync state.
- `events` stores decoded contract events with ledger and transaction context.
- `invocations` stores per-call CPU instructions, memory, ledger I/O and fees.
- `storage_entries` stores temporary, persistent and instance storage snapshots.
- `sync_state` stores per-contract and per-network indexing cursors.

Events and invocations are partitioned by month. Queries take a contract ID
and optional ledger or time bounds; pagination uses opaque cursors.

## API

The Go API reads from Postgres and exposes versioned REST routes under
`/api/v1`. Every route is documented in `docs/openapi.yaml`, and the generated
TypeScript types used by this dashboard come from that same document.

Authentication is optional for read endpoints. Project API keys carry scopes;
workspace session tokens identify per-user resources such as watchlists,
groups and saved rules.

## Watchdog

The watchdog is a Soroban contract plus an off-chain monitor. Contracts
registered with it publish health checks on-chain. Sorolens indexes those
checks and turns them into uptime history, alerts and SLA reports.

Watchdog data is additive: a contract can be indexed without being monitored,
and the dashboard keeps the two views separate.
