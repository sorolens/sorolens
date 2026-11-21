# Zero-downtime migration policy

The `events` table is large, partitioned by month and written to continuously
by the indexer, while the API reads it on every request. A migration that
rewrites it or holds a long lock stalls both. Every migration in
`apps/api/internal/db/migrations` must keep the running API and indexer
working while it applies, and while old and new code run side by side during a
deploy.

The rules below are enforced by `TestMigrationLint`
(`apps/api/internal/db/migration_lint_test.go`). It runs inside the existing Go
CI job as part of `go test ./...`, and locally with `make migrate-check`. A
violation fails CI with the rule, the offending statement and the safe
alternative.

A rule does not apply to a table that the same migration creates: a new table
is empty and unused, so locking or rewriting it costs nothing.

## Rules

| Rule | Unsafe | Why | Safe alternative |
|---|---|---|---|
| `index-not-concurrent` | `CREATE INDEX idx ON events (ledger);` | Blocks writes for the whole build. | `CREATE INDEX CONCURRENTLY IF NOT EXISTS idx ON events (ledger);` in its own migration. |
| `add-column-not-null-without-default` | `ALTER TABLE contracts ADD COLUMN kind TEXT NOT NULL;` | Fails on a non-empty table. | Add it with a constant `DEFAULT` (a metadata-only change since PostgreSQL 11), or nullable, then backfill and constrain. |
| `alter-column-type` | `ALTER TABLE events ALTER COLUMN ledger TYPE BIGINT;` | Rewrites the table under an `ACCESS EXCLUSIVE` lock. | Expand and contract (below). |
| `set-not-null` | `ALTER TABLE events ALTER COLUMN tx_hash SET NOT NULL;` | Scans every row while holding an `ACCESS EXCLUSIVE` lock. | `ADD CONSTRAINT c CHECK (tx_hash IS NOT NULL) NOT VALID;` then, in a later migration, `VALIDATE CONSTRAINT c;` and only then `SET NOT NULL` (PostgreSQL skips the scan once a valid check exists). |
| `constraint-without-not-valid` | `ALTER TABLE events ADD CONSTRAINT fk FOREIGN KEY (contract_id) REFERENCES contracts (id);` | Validates every row while blocking writes. | Add it `NOT VALID`, then `VALIDATE CONSTRAINT` in a separate migration (validation takes a weaker lock). |
| `rename` | `ALTER TABLE events RENAME COLUMN ledger TO ledger_seq;` | Old instances still use the old name during a deploy. | Expand and contract. |
| `drop-column` | `ALTER TABLE contracts DROP COLUMN legacy;` | Old instances still read or write it during a deploy. | Ship code that no longer uses the column first; drop it in a later release. |
| `full-table-lock` | `VACUUM FULL events;`, `CLUSTER events;`, `LOCK TABLE events;` | Holds an `ACCESS EXCLUSIVE` lock. | Plain `VACUUM`, `pg_repack`, or a design that needs no explicit lock. |

## Expand and contract

Renames and type changes are split across releases so every deployed version
of the code finds the schema it expects:

1. **Expand:** add the new column (nullable, or with a constant default).
2. Deploy code that writes both columns, then backfill the new column in
   batches (`UPDATE … WHERE id BETWEEN …`), never in one statement over
   `events`.
3. Deploy code that reads the new column.
4. **Contract:** in a later release, stop writing the old column and drop it.

## Indexes on the partitioned `events` table

PostgreSQL does not support `CREATE INDEX CONCURRENTLY` on a partitioned
parent. To index `events` without blocking writes:

```sql
-- 1. An invalid index on the parent only; this is instant.
CREATE INDEX IF NOT EXISTS idx_events_x ON ONLY events (x);
-- 2. For each partition, build concurrently (one migration per statement
--    if needed, since CONCURRENTLY cannot run inside a transaction):
CREATE INDEX CONCURRENTLY IF NOT EXISTS events_2026_11_x_idx ON events_2026_11 (x);
-- 3. Attach each partition's index; the parent index becomes valid once all are attached.
ALTER INDEX idx_events_x ATTACH PARTITION events_2026_11_x_idx;
```

## Exceptions

When a flagged statement is genuinely safe, for example on a small
configuration table, add a line to
`apps/api/internal/db/migration_lint_allowlist.txt`:

```
<file> <rule> <reason>
```

The reason is required and is what a reviewer reads, so say why the statement
is safe *here*. Entries that no longer match a finding fail the test, so the
list cannot go stale. Existing migrations that predate this policy are listed
there with their reasons; they are already applied everywhere and must never be
edited to satisfy the linter.

## Down migrations

Every `*.up.sql` must have a matching `*.down.sql`, and versions must be unique
and contiguous; `TestMigrationVersionsAreUniqueAndContiguous` enforces this in
the same CI job.
