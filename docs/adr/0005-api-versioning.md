# 0005. API versioning and /api/v1 freeze

## Status

Accepted (retroactive) — 2026-09-26

## Context

The initial `/api/v1` surface grew incrementally, resulting in minor response inconsistencies: resource list envelopes used distinct keys (`contracts`, `events`, `alerts`), some endpoints lacked cursor pagination, timestamp formats varied, and absent optional fields were inconsistently omitted or returned as empty strings. However, third-party applications and SDKs depend on v1 remaining stable.

## Decision

Freeze `/api/v1` and introduce a clean `/api/v2` namespace:

1. **v1 Freeze:** The `/api/v1` surface is strictly frozen against breaking changes. CI enforces this on pull requests using `oasdiff` against the base commit whenever `docs/openapi.yaml` changes.
2. **v2 Conventions:** The `/api/v2/*` namespace runs alongside `/api/v1/*` with standardized conventions:
   - Unified list envelope: `{"data": [...], "pagination": {"next_cursor": "...", "has_more": true}}`.
   - Universal keyset pagination across all list routes.
   - Strict RFC 3339 UTC timestamps (no unix epoch or non-UTC offsets).
   - Explicit `null` for missing optional values (never omitted or empty strings).
   - Uniform error envelope: `{"error": {"code", "message", "request_id"}}` matching v1 status codes.
3. **Route parity:** Every v1 route has an equivalent v2 endpoint, enforced by the unit test `TestV2CoversEveryV1Route`.

## Consequences

- **Positive:** Existing v1 clients avoid breaking changes; v2 establishes consistent ergonomics for client libraries; automated CI guards prevent inadvertent API drift.
- **Negative:** Requires running and testing dual handler layers in the API router over shared database stores.

## Alternatives considered

- Header-based versioning (`Accept: application/vnd.sorolens.v2+json`): Rejected in favor of path-based versioning for URL transparency, simple reverse proxy caching, and ease of debugging.

## Sources

- Issues: [#144](https://github.com/sorolens/sorolens/issues/144), [#553](https://github.com/sorolens/sorolens/issues/553)
- Pull requests: [#334](https://github.com/sorolens/sorolens/pull/334) (commit `642c103`), [#781](https://github.com/sorolens/sorolens/pull/781) (commit `5946fcf`)
- Documentation: [`docs/api-v2.md`](https://github.com/sorolens/sorolens/blob/main/docs/api-v2.md), [ROADMAP.md](https://github.com/sorolens/sorolens/blob/main/ROADMAP.md#v04---ecosystem-integration-november-2026)
- Code: [`apps/api/internal/handler/v2.go`](https://github.com/sorolens/sorolens/blob/main/apps/api/internal/handler/v2.go), [`.github/workflows/ci.yml`](https://github.com/sorolens/sorolens/blob/main/.github/workflows/ci.yml#L159-L184)

## Open questions for maintainers

- Rationale not documented for a formal deprecation schedule or eventual sunset date for the frozen `/api/v1` namespace.
