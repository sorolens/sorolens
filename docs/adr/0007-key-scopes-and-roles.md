# 0007. Two-tier authentication: API key scopes and user roles

## Status

Accepted (retroactive) — 2026-09-24

## Context

Sorolens serves automated systems (CLI, indexers, bots) and multi-user dashboard teams. Multi-user deployments require role separation (viewers vs operators), and automation tokens require least-privilege scoping without human user profiles.

## Decision

Implement a two-tier access pipeline separating Key Scopes from User Roles:

1. **Identity Resolution:** Middleware `Identity` (`internal/middleware/identity.go`) verifies credentials and attaches a `VerifiedIdentity` context:
   - Workspace session tokens: HMAC-SHA256 tokens (`sls_...`) issued by `POST /api/v1/auth/session`.
   - Scoped API keys: `sl_...` tokens looked up by hash, optionally bound to a user (`api_keys.user_id`).
   - Trusted proxy signatures: `X-Identity-*` headers signed via HMAC-SHA256. Raw identity headers are strictly ignored (issue #518).
2. **Scope Enforcement:** Middleware `ValidateScope` (`internal/middleware/scopes.go`) matches routes against a chi route metadata table:
   - Scopes: `read:contracts`, `write:contracts`, `read:watchdog`, and `admin:*`.
   - Missing required scopes return HTTP 403. Public read endpoints remain open if unauthenticated.
3. **Role Enforcement:** Middleware `RequireRole` (`internal/middleware/roles.go`) enforces a hierarchical role check (`viewer` < `contributor` < `admin`):
   - `viewer`: Read-only queries.
   - `contributor`: Tracks, untracks, or tags contracts.
   - `admin`: Manages API keys, audit logs, and runtime profiling (`/debug/pprof/*`).
   - An `admin:*` key satisfies admin role checks without a user binding.

## Consequences

- **Positive:** Granular least privilege for API keys; clear hierarchical permissions for user sessions; safe bootstrapping with admin keys.
- **Negative:** Two-stage authorization logic across handlers.

## Alternatives considered

- Pure RBAC without scopes: Rejected because automation tokens need least-privilege scoping without requiring a full user profile.

## Sources

- Issues: [#108](https://github.com/sorolens/sorolens/issues/108), [#133](https://github.com/sorolens/sorolens/issues/133), [#518](https://github.com/sorolens/sorolens/issues/518)
- PRs: [#278](https://github.com/sorolens/sorolens/pull/278) (commit `34c11e9`), [#290](https://github.com/sorolens/sorolens/pull/290) (commit `1cbc5a1`)
- Migrations: [`000007_api_key_scopes.up.sql`](https://github.com/sorolens/sorolens/blob/main/apps/api/internal/db/migrations/000007_api_key_scopes.up.sql), [`000012_user_roles.up.sql`](https://github.com/sorolens/sorolens/blob/main/apps/api/internal/db/migrations/000012_user_roles.up.sql), [`000036_api_key_user_id.up.sql`](https://github.com/sorolens/sorolens/blob/main/apps/api/internal/db/migrations/000036_api_key_user_id.up.sql)
- Code: [`apps/api/internal/middleware/identity.go`](https://github.com/sorolens/sorolens/blob/main/apps/api/internal/middleware/identity.go), [`apps/api/internal/middleware/scopes.go`](https://github.com/sorolens/sorolens/blob/main/apps/api/internal/middleware/scopes.go), [ARCHITECTURE.md](https://github.com/sorolens/sorolens/blob/main/ARCHITECTURE.md#authentication-and-scopes)

## Open questions for maintainers

- Rationale not documented for whether custom roles or tenant permissions are planned beyond viewer, contributor, and admin.
