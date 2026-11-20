# 0004. S3-compatible Parquet cold storage for historical events

## Status

Accepted (retroactive) — 2026-09-26

## Context

Storing all historical smart contract events in relational Postgres storage is cost-prohibitive at scale. However, historical event records must remain queryable for protocol audits and analyses without full database expansion.

## Decision

Archive events older than a configurable threshold to an S3-compatible object store using Apache Parquet:

1. **Archival lifecycle:** A scheduled job (`apps/api/cmd/coldarchive`) queries contracts with events older than `COLD_STORAGE_THRESHOLD_DAYS` (default 90 days).
2. **Object layout:** Cold events are exported in batches (default 5,000) and grouped by month under `events/{contract_id}/{year}-{month}.parquet` in an `ObjectStore` (AWS S3, MinIO, or Cloudflare R2). Existing month files are merged with newly cold events (`Archiver.exportByMonth`).
3. **Safety:** Data is written and verified in object storage before rows are deleted from Postgres. An upload failure aborts the contract batch, preserving hot data.
4. **Transparent retrieval:** `coldstorage.Reader` queries Parquet objects when an API request targets an older ledger range absent from Postgres.

## Consequences

- **Positive:** Reduces storage costs by ~90% compared to Postgres; Parquet columnar encoding compresses data effectively; historical queries continue to succeed.
- **Negative:** Cold queries have higher latency than Postgres index seeks; requires external S3-compatible bucket configuration and credentials.

## Alternatives considered

- Embedded DuckDB querying: Suggested in issue #146, but replaced with a pure Go Parquet decoder (`coldstorage.Reader`) to avoid CGo/C++ dependencies.

## Sources

- Issue: [#146](https://github.com/sorolens/sorolens/issues/146)
- Pull request: [#334](https://github.com/sorolens/sorolens/pull/334) (commit `642c103`)
- Code: [`apps/api/cmd/coldarchive/main.go`](https://github.com/sorolens/sorolens/blob/main/apps/api/cmd/coldarchive/main.go), [`apps/api/internal/coldstorage/archive.go`](https://github.com/sorolens/sorolens/blob/main/apps/api/internal/coldstorage/archive.go), [`apps/api/internal/coldstorage/parquet.go`](https://github.com/sorolens/sorolens/blob/main/apps/api/internal/coldstorage/parquet.go), [`apps/api/internal/coldstorage/reader.go`](https://github.com/sorolens/sorolens/blob/main/apps/api/internal/coldstorage/reader.go)

## Open questions for maintainers

- Rationale not documented for how object-store Parquet archives interact with the in-database `events_archive` table from migration 000041 (whether `events_archive` serves as a temporary buffer or an alternative tier).
