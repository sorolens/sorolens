# 0002. Keyset cursor pagination for API feeds

## Status

Accepted (retroactive) — 2026-07-26

## Context

The API serves high-volume event feeds, invocation traces, and watchdog alerts appended continually by the indexer. Standard `OFFSET`/`LIMIT` pagination suffers from row drift (skipped or duplicated rows when inserts occur between requests) and $O(N)$ scan penalties on deep pages.

## Decision

Adopt keyset cursor pagination across all list endpoints:

1. **Opaque encoding:** Cursors are standard base64 strings wrapping natural sort tuples:
   - Invocations: base64 of `<ledger>:<tx_hash>` parsed by `cursor.ParseInvocation`.
   - Events: base64 of `{ledger}:{id}` or RPC event ID token.
   - Watchdog alerts and alert groups: base64 of `RFC3339Nano|contract_id` or `RFC3339Nano|id`.
2. **Keyset queries:** Repositories filter rows using index comparisons (e.g. `(ledger, id) < (cursor_ledger, cursor_id)` with `DESC` ordering) and fetch `limit + 1` rows to determine `has_more` and `next_cursor`.
3. **Strict decoding:** Package `apps/api/internal/cursor` treats cursor parsing as security-adjacent; malformed tokens return HTTP 422 (`ErrInvalidCursor`) rather than defaulting to the start of the list.

## Consequences

- **Positive:** Paginating feeds remains stable during ingestion; index seeks remain $O(1)$ regardless of depth; matches Soroban RPC `getEvents` cursor semantics.
- **Negative:** Clients cannot jump directly to arbitrary page offsets; reverse traversal requires distinct query predicates.

## Alternatives considered

- Offset/Limit pagination: Rejected due to row drift and excessive scan latency on large tables.
- Rationale not documented for why different store packages use different delimiter characters (`:` vs `|`) before base64 encoding rather than a unified format.

## Sources

- Issues: [#150](https://github.com/sorolens/sorolens/issues/150), [#849](https://github.com/sorolens/sorolens/issues/849)
- Pull requests: [#3](https://github.com/sorolens/sorolens/pull/3) (commit `1fafa2f`), [#420](https://github.com/sorolens/sorolens/pull/420) (commit `0d22a30`), [#739](https://github.com/sorolens/sorolens/pull/739) (commit `da924ed`)
- Code: [`apps/api/internal/cursor/cursor.go`](https://github.com/sorolens/sorolens/blob/main/apps/api/internal/cursor/cursor.go), [`apps/api/internal/store/alert_groups.go`](https://github.com/sorolens/sorolens/blob/main/apps/api/internal/store/alert_groups.go), [ARCHITECTURE.md](https://github.com/sorolens/sorolens/blob/main/ARCHITECTURE.md#4-rest-api-apiv1)

## Open questions for maintainers

- Rationale not documented for maintaining colon-delimited (`<ledger>:<tx_hash>`) and pipe-delimited (`<timestamp>\|<id>`) cursor formats instead of a single shared serialization structure.
