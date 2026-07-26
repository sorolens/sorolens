# ROADMAP.md
## Sorolens Release Roadmap

---

## v0.1 - "It Indexes" (August 2026)

**Theme:** Get data flowing. Ship a working indexer, a queryable API, and a minimal dashboard that proves the core loop works on testnet.

**Scope:** Everything in this milestone is built solo, targeting the next two weeks.

### Shipped capability

**Monorepo and tooling:**
- `go.work` monorepo: `api`, `indexer`, `cli` modules.
- `pnpm` workspace: `dashboard` (Next.js 15), `xdr` (TypeScript XDR decoder package).
- Postgres 16 schema (as defined in ARCHITECTURE.md) applied via `golang-migrate`.
- Redis (Upstash) configured with `go-redis`.
- GitHub Actions cron workflow running the indexer binary on a 5-minute schedule.

**Indexer (`indexer/`):**
- Redis advisory lock: global run lock prevents concurrent indexer runs.
- `getLatestLedger` call to establish cursor position.
- `getEvents` polling with cursor pagination for tracked contracts.
- Upsert events into `events` table.
- `getTransaction` fetch for each unique `txHash` in event batches; upsert into `invocations` table.
- `getLedgerEntries` snapshot of contract instance and known storage keys; upsert into `storage_entries` table.
- Backfill path: when a contract's status is `"backfilling"`, fetch up to 7 days of prior events before switching to `"active"`.
- Sync state written to `sync_state` table after each run.

**API (`api/`):**
- Go with `chi` router, `pgx` pool, deployed as Vercel Go serverless functions.
- `POST /api/v1/contracts` (track a contract).
- `GET /api/v1/contracts` (list tracked contracts).
- `GET /api/v1/contracts/:id` (contract detail with sync state).
- `GET /api/v1/contracts/:id/events` (paginated events, cursor-based).
- `GET /api/v1/contracts/:id/invocations` (paginated invocations).
- `GET /api/v1/contracts/:id/storage` (current storage snapshot with TTL data).
- `GET /api/v1/stats/global` (global summary).
- All responses use `Content-Type: application/json`; errors return structured JSON.

**Dashboard (`dashboard/`):**
- Next.js 15 app router with Tailwind CSS.
- Landing page: list of tracked contracts with status and last-sync time.
- Contract detail page: tabbed view of Events, Invocations, Storage.
- Events tab: scrollable feed with cursor-based load-more. Decoded topics and values where available.
- Invocations tab: table showing tx_hash, function, status, resource fee, CPU. Link to transaction on Stellar Expert.
- Storage tab: table of entries with durability, TTL (ledgers remaining), and live/archived badge.
- No auth (all public for v0.1).

**XDR decoder (`packages/xdr/`):**
- TypeScript package wrapping `@stellar/stellar-sdk` XDR methods.
- Exports `decodeScVal(base64: string): unknown` and `decodeTopics(base64Array: string[]): unknown[]`.
- Used by the API to populate `topic_decoded` and `value_decoded` columns.

**CLI (`cli/`):**
- `sorolens track <contract-id> --network testnet` (calls the API to register a contract).
- `sorolens events <contract-id>` (prints recent events to stdout as JSON).
- `sorolens storage <contract-id>` (prints storage entries with TTL).

**Rust fixture contract (`contracts/`):**
- A simple Soroban counter contract that emits events on increment.
- Used for local integration testing of the indexer.
- Deployed to testnet for demo purposes.

**Documentation:**
- README.md with setup instructions, environment variable reference, and a 5-minute quickstart.
- `CONTRIBUTING.md` with repo structure and how to run tests.

---

## v0.2 - "It's Useful" (September 2026)

**Theme:** Add the features that turn the project from a demo into a tool a protocol developer would actually use. Open the project for external contributions.

**Obvious contributor entry points:**
- Additional XDR decoding coverage (custom ScVal types, contract-specific ABIs).
- New filter options on the events and invocations endpoints.
- Additional CLI commands.
- Dashboard UI improvements (charts, filtering, search).

### Shipped capability

**Indexer improvements:**
- Decode `diagnosticEventsXdr` from `getTransaction` to extract `function_name` and `core_metrics` resource counters (CPU, memory, ledger bytes) into the `invocations` table. (This requires a public RPC node that enables diagnostic events, or a self-hosted node.)
- Configurable fallback: if diagnosticEventsXdr is empty, attempt to derive `function_name` from the `fn_call` topic in the contract events.
- Storage expiry alerting: after each indexer run, check if any `storage_entries.ledgers_until_expiry <= 5000` (approximately 7 days). Write a summary to a configurable webhook (Discord/Slack JSON payload).

**API additions:**
- `GET /api/v1/contracts/:id/invocations/:tx_hash` (full invocation detail with associated events).
- `GET /api/v1/contracts/:id/invocations/stats` (aggregated resource usage: avg/p95 CPU, avg fee, failure rate over a time window).
- Webhook registration: `POST /api/v1/contracts/:id/webhooks` so external systems can subscribe to new events or TTL alerts.

**Dashboard additions:**
- Resource usage charts on the contract detail page: CPU instructions and resource fee over time (line chart, last 24h / 7d / 30d).
- Failure rate badge on the Invocations tab.
- Storage health panel: counts of entries by durability and status, with a highlight for entries expiring within 7 days.
- Decoded argument display for invocations where function_name is known.

**Docs and contributor tooling:**
- GitHub issue templates: bug report, feature request, new-decoder-type.
- At least 5 good-first-issues filed covering XDR decoder coverage gaps, filter improvements, and CLI additions.
- `MAINTAINER.md` with the application process and what being a Sorolens maintainer means.

---

## v0.3 - "Multi-Contract and Mainnet" (October 2026)

**Theme:** Make Sorolens useful for protocol teams tracking multiple contracts in production. Prove it works on mainnet with real data.

**Obvious contributor entry points:**
- Cross-contract invocation graph (requires contributor familiar with graph visualization).
- Hubble (BigQuery) integration for historical data beyond 7-day RPC window.
- Additional language bindings for the CLI (Python SDK-based alternative).
- Performance testing and query optimization for high-event-volume contracts.

### Shipped capability

**Multi-contract view:**
- Dashboard home page shows aggregated activity across all tracked contracts: global event volume chart, global failure rate, top-active contracts by invocation count.
- `GET /api/v1/events` (global feed across all contracts, same cursor pagination pattern).
- Cross-contract invocation graph: when a contract calls another tracked contract, show the relationship in the contract detail page.

**Mainnet support:**
- API and indexer support `network = "mainnet"` as a first-class option.
- Default mainnet RPC endpoint configurable via environment variable.
- Dashboard network selector (testnet / mainnet).
- Documentation updated with mainnet setup instructions.

**Retention policy:**
- Configurable event retention window. Events older than N days are archived to a `events_archive` table (not deleted). The API `since`/`until` params still work against archived data.
- Cron job to move rows older than retention threshold.

**CLI improvements:**
- `sorolens tail <contract-id>` (live-tail events, polling loop, prints decoded events as they arrive).
- `sorolens inspect <tx-hash>` (show full invocation detail for a transaction: status, function, args, resource breakdown).
- `sorolens health <contract-id>` (show storage health: entries by durability, entries near expiry, suggested extend_ttl commands).

---

## v0.4 - "Ecosystem Integration" (November 2026)

**Theme:** Make Sorolens easy to embed in third-party workflows. Webhooks are stable, the API is versioned, and there are at least two integrations available.

**Obvious contributor entry points:**
- OpenZeppelin Monitor integration (map Sorolens webhook payload to OpenZeppelin Monitor triggers).
- Grafana datasource plugin.
- GitHub Actions action: `sorolens-check` that fails CI if any tracked contract has storage entries expiring within N ledgers.
- Additional contract-type templates (DEX, lending protocol, stablecoin) with pre-configured event filters and named function decoders.

### Shipped capability

**Webhook system v2:**
- Webhook signature verification (HMAC-SHA256, same pattern as GitHub webhooks).
- Retry logic with exponential backoff for failed webhook deliveries.
- Webhook delivery log accessible via `GET /api/v1/webhooks/:id/deliveries`.

**API v2 compatibility note:**
- `/api/v1` surface is frozen. No breaking changes to existing endpoints.
- New capabilities added under `/api/v1` only if backwards-compatible.
- If a breaking change is needed, it goes to `/api/v2` with a documented migration guide.

**Integrations shipped:**
- Grafana datasource plugin (contributed by external maintainer or bounty).
- GitHub Actions action `sorolens-check` for CI TTL health assertions.
- Documentation page: "How to monitor your contract with Sorolens + OpenZeppelin Monitor."

**Performance:**
- Query optimization for contracts with > 1M events in the database.
- Connection pooling via PgBouncer (or Neon's built-in pooling) documented and tested.
- Indexer parallelism: concurrent RPC calls for multiple contracts in the same run, guarded by a semaphore.
