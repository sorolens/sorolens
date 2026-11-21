# 0001. Postgres partitioning for events table

## Status

Accepted (retroactive) — 2026-09-23

## Context

At sustained mainnet volume, the `events` table accumulates hundreds of millions of rows. Monolithic table scans, B-tree index maintenance, and bulk deletions degrade query latency and cause disk growth. Operational queries predominantly filter by contract and time bounds.

## Decision

Partition the `events` table by month using Postgres declarative range partitioning on `ledger_closed_at`:

1. **Partition scheme:** Table `events` uses `PARTITION BY RANGE (ledger_closed_at)` with monthly child tables (`events_YYYY_MM`).
2. **Primary key:** Uses composite primary key `(id, ledger_closed_at)` as required by Postgres for partitioned unique constraints.
3. **Partition provisioning:** Pre-provisions monthly partitions up to 6 months in advance via `maintain_events_partitions()`, run by indexer cycles and the retention job (`cmd/retention`).
4. **Default partition:** Migration 000040 adds `events_default` so historical backfills and future events outside provisioned ranges insert without error.
5. **Retention:** Function `archive_old_events()` moves events exceeding `EVENTS_RETENTION_DAYS` (default 90 days) into `events_archive`. The view `events_with_archive` unifies live and archived events.

## Consequences

- **Positive:** Partition pruning accelerates time-bounded contract queries; old partitions detach or drop cleanly without table locks.
- **Negative:** Queries without time bounds scan multiple partitions; all foreign keys and unique constraints must include `ledger_closed_at`.

## Alternatives considered

- `pg_partman`: Discussed in issue #145, but native PL/pgSQL functions were chosen to avoid non-standard extension dependencies in managed environments like Neon.
- Hash partitioning on `contract_id`: Rejected because event volume follows time-series lifecycles.

## Sources

- Issues: [#145](https://github.com/sorolens/sorolens/issues/145)
- Pull requests: [#281](https://github.com/sorolens/sorolens/pull/281) (commit `57520ba`), [#856](https://github.com/sorolens/sorolens/pull/856) (commit `521344f`)
- Migrations: [`000005_partition_events.up.sql`](https://github.com/sorolens/sorolens/blob/main/apps/api/internal/db/migrations/000005_partition_events.up.sql), [`000040_events_default_partition.up.sql`](https://github.com/sorolens/sorolens/blob/main/apps/api/internal/db/migrations/000040_events_default_partition.up.sql), [`000041_events_retention_archive.up.sql`](https://github.com/sorolens/sorolens/blob/main/apps/api/internal/db/migrations/000041_events_retention_archive.up.sql)
- Code: [`apps/api/cmd/retention/main.go`](https://github.com/sorolens/sorolens/blob/main/apps/api/cmd/retention/main.go), [`services/indexer/internal/partition/partition.go`](https://github.com/sorolens/sorolens/blob/main/services/indexer/internal/partition/partition.go), [ARCHITECTURE.md](https://github.com/sorolens/sorolens/blob/main/ARCHITECTURE.md#3-postgres-schema-ddl)

## Open questions for maintainers

- Rationale not documented for whether `events_archive` rows will eventually be pruned or offloaded to object storage after an extended secondary retention period.
