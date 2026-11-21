# Architecture Decision Records

This directory records the significant architecture and design decisions made in Sorolens.

## When to write an ADR

Write an Architecture Decision Record when a proposed change meets any of these criteria:

- **Hard to reverse:** The decision involves data models, storage layouts, public API contracts, or protocols that would be costly or breaking to undo later.
- **Cross-component impact:** The change affects more than one subsystem (for example, both the indexer poller and the REST API, or both API schemas and SDK generators).
- **Newcomer questions:** The architectural choice addresses a problem where future contributors or maintainers would otherwise ask "why is it done this way?"
- **Not for routine changes:** Routine bug fixes, small feature additions, dependency bumps, or internal refactors that do not alter system design or contracts do not require an ADR.

## Process and conventions

- **Numbering:** Records use sequential four-digit numbers prefixed with zeros (`0001`, `0002`, etc.). New proposals copy [`0000-template.md`](0000-template.md) to the next available number.
- **Superseding records:** An existing ADR is never deleted or rewritten to represent new architecture. When a decision is replaced, update the old record's Status to `Superseded by [ADR-XXXX](XXXX-...)` and note in the new record's Context which ADR it supersedes.
- **Linking:**
  - Link between ADRs and docs pages using relative Markdown links (`[ADR-0001](0001-postgres-partitioning.md)`).
  - Link to source code, migrations, workflows, and repository files outside `docs/` using full GitHub URLs (`https://github.com/sorolens/sorolens/blob/main/...`), in accordance with [documentation link conventions](../contributing/docs.md#add-or-change-a-page).

## Decision records index

| Number | Title | Status | Date |
| --- | --- | --- | --- |
| [0001](0001-postgres-partitioning.md) | Postgres partitioning for events table | Accepted (retroactive) | 2026-09-23 |
| [0002](0002-cursor-pagination.md) | Keyset cursor pagination for API feeds | Accepted (retroactive) | 2026-07-26 |
| [0003](0003-indexer-sharding.md) | Horizontally-scaled indexer with worker sharding | Accepted (retroactive) | 2026-09-25 |
| [0004](0004-cold-storage.md) | S3-compatible Parquet cold storage for historical events | Accepted (retroactive) | 2026-09-26 |
| [0005](0005-api-versioning.md) | API versioning and /api/v1 freeze | Accepted (retroactive) | 2026-09-26 |
| [0006](0006-sse-vs-websocket.md) | Server-Sent Events (SSE) and WebSocket protocol selection | Accepted (retroactive) | 2026-09-28 |
| [0007](0007-key-scopes-and-roles.md) | Two-tier authentication: API key scopes and user roles | Accepted (retroactive) | 2026-09-24 |
| [0008](0008-sdk-generation.md) | OpenAPI spec as source of truth and automated SDK generation | Accepted (retroactive) | 2026-09-24 |
