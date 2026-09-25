# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
