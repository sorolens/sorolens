# Issue closure summary

This branch consolidates the requested issue closures for the project backlog:

- Closes #95: watchdog telemetry is now wired through the indexer poller and classified in real time.
- Closes #109: API rate limiting and API-key auth are enforced with Redis-backed counters and key management.
- Closes #138: performance baselines and regression alerts are computed and emitted idempotently.
- Closes #147: backup and restore scripts plus the PITR runbook are in place.

Validation status:

- `go test ./services/indexer/... ./apps/api/...` is passing.
- Backup and restore scripts are documented in the repo and available under `scripts/`.
