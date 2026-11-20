# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `sorolens doctor` checks the config file and active profile, API reachability and TLS, the API key and its scopes, server version compatibility and clock skew, printing pass, warn or fail with a fix hint for each. `--json` gives a stable, documented report, secrets are redacted from all output, and the command exits non-zero when a check fails (#815).
- Automated GitHub Actions release workflow (`release.yml`) for publishing container images to GHCR and creating GitHub Releases with SHA256 checksums on semver tag push (#544).
- Release process documentation in `CONTRIBUTING.md`.
- The browser extension is now discoverable from the product: a callout on the landing page and a **Browser extension** card in Settings that mints a key scoped to exactly the endpoints the extension calls (`read:contracts`, `write:contracts`) for pairing (#764).
- `extension/README.md` documents the pairing contract — the API key is the credential, and since #518 a bare `X-User-ID` header grants nothing. The old "copy `sorolens_user_id` out of localStorage" instructions, which had stopped working, are gone (#764).

### Changed

- One site URL source for the dashboard (#761): `apps/web/lib/siteUrl.ts` reads `NEXT_PUBLIC_APP_URL` (one variable, one fallback) and now drives `metadataBase`, `robots.txt` and `sitemap.xml`, which previously read three different things — `NEXT_PUBLIC_SITE_URL` with a localhost fallback in the sitemap, `NEXT_PUBLIC_APP_URL` with a production fallback in robots, and `NEXT_PUBLIC_APP_URL` again in the root layout. `NEXT_PUBLIC_SITE_URL` is no longer read; deployments that set only it must rename it to `NEXT_PUBLIC_APP_URL`, or the sitemap falls back to `https://sorolens.xyz`. The sitemap now lists every public, non-personal route plus one entry per tracked contract (page-followed from `GET /api/v1/contracts`), and stamps entries with the API's `last_activity_at`/`added_at` instead of a fresh `new Date()` per request. `/events` no longer describes itself as the SSE stream — that is `/events/live`, which got its own metadata.
- Canonical domain consistency (#569): Standardized on canonical domain `sorolens.xyz` (`https://sorolens.xyz` for the web dashboard and `https://api.sorolens.xyz` for the API) across all documentation, metadata, configurations, and defaults, eliminating deprecated domain references (such as legacy `.dev`, Vercel staging preview, and `.com` endpoints). Added a CI grep guard to prevent regressions.
- Landing-page live stats state what they are: a loading skeleton while the first figures are in flight, an explicit age on any figure that is stale, and a distinct unavailable message when nothing could be read. A failed read is never rendered as a zero, and cached figures are no longer presented as current (#764).

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
