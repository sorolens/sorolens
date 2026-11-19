# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Automated GitHub Actions release workflow (`release.yml`) for publishing container images to GHCR and creating GitHub Releases with SHA256 checksums on semver tag push (#544).
- Release process documentation in `CONTRIBUTING.md`.

### Changed

- One site URL source for the dashboard (#761): `apps/web/lib/siteUrl.ts` reads `NEXT_PUBLIC_APP_URL` (one variable, one fallback) and now drives `metadataBase`, `robots.txt` and `sitemap.xml`, which previously read three different things — `NEXT_PUBLIC_SITE_URL` with a localhost fallback in the sitemap, `NEXT_PUBLIC_APP_URL` with a production fallback in robots, and `NEXT_PUBLIC_APP_URL` again in the root layout. `NEXT_PUBLIC_SITE_URL` is no longer read; deployments that set only it must rename it to `NEXT_PUBLIC_APP_URL`, or the sitemap falls back to `https://sorolens.dev`. The sitemap now lists every public, non-personal route plus one entry per tracked contract (page-followed from `GET /api/v1/contracts`), and stamps entries with the API's `last_activity_at`/`added_at` instead of a fresh `new Date()` per request. `/events` no longer describes itself as the SSE stream — that is `/events/live`, which got its own metadata.

## [0.1.0] - 2026-09-25

### Added

- Initial release.
- Contract event indexing: every event a tracked Soroban contract emits, decoded and stored in Postgres, queryable through the REST API.
- Invocation tracing with per-transaction CPU instructions, memory, ledger I/O bytes, and fees charged.
- Storage tracking: snapshots of temporary, persistent, and instance storage entries with TTL health.
- On-chain `sorolens-watchdog` contract integration for proactive contract monitoring and alerting.
- Alert subscriptions for storage-expiry and contract-status transitions.
- Multi-network support (testnet, mainnet, and futurenet) with a `?network=` filter on list endpoints.
- Snapshot / replay endpoints for reconstructing contract state at a historical ledger.
- TypeScript SDK, CLI, and dashboard packages under `packages/` and `apps/`.
- Rust watchdog contract under `contracts/watchdog`.

[Unreleased]: https://github.com/sorolens/sorolens/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/sorolens/sorolens/releases/tag/v0.1.0
