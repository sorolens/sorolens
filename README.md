[![CI](https://github.com/sorolens/sorolens/actions/workflows/ci.yml/badge.svg)](https://github.com/sorolens/sorolens/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Release](https://img.shields.io/github/v/release/sorolens/sorolens)](https://github.com/sorolens/sorolens/releases)
# Sorolens
Indexed observability for Soroban smart contracts: events, invocations, resource metrics, and storage TTL health in one place.
---
## The problem
Soroban's public RPC retains events for 24 hours and transaction data for up to 7 days. There is no persistent index of what your contract did, what it cost, or which storage entries are close to expiring. The Stellar Lab contract explorer is excellent for point-in-time inspection but has no history, no API, and no alerting. Sorolens fills that gap.
---
## What it does
- Indexes events, invocations, and storage entries for any tracked Soroban contract into Postgres.
- Exposes a REST API (`/api/v1`) queryable by your backend or CI pipeline.
- Shows decoded event topics and values, per-invocation resource metrics (CPU instructions, memory, ledger bytes, fee charged), and storage TTL health in a Next.js dashboard.
- Alerts when persistent storage entries are within a configurable number of ledgers of expiry.
---
## Screenshots
<!-- SCREENSHOT PLACEHOLDER: dashboard home page showing tracked contracts, global event volume chart, and failure rate -->
> `docs/screenshots/dashboard-home.png`
<!-- SCREENSHOT PLACEHOLDER: contract detail page, Events tab, with decoded transfer events -->
> `docs/screenshots/contract-events.png`
<!-- SCREENSHOT PLACEHOLDER: contract detail page, Storage tab, with TTL health panel and expiring-soon highlight -->
> `docs/screenshots/contract-storage.png`
<!-- SCREENSHOT PLACEHOLDER: CLI output of `sorolens tail <contract-id>` showing a live event stream -->
> `docs/screenshots/cli-tail.png`
---
## Quickstart
### Prerequisites
- Docker and Docker Compose
- Go 1.23+
- Node 22+ and pnpm 9+
### 1. Start local dependencies
```bash
docker compose up -d
```
This starts Postgres 16 and Redis locally. The `docker-compose.yml` at the repo root maps Postgres to `localhost:5432` and Redis to `localhost:6379`.
### 2. Apply the database schema
```bash
cd services/indexer
go run ./cmd/migrate up
```
### 3. Track a contract and run the indexer
```bash
# Register a contract (uses the local API)
go run ./cmd/sorolens track CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC --network testnet
# Run one indexer cycle manually
go run ./cmd/sorolens index --once
```
The dashboard is at `http://localhost:3000` after `pnpm dev` in `apps/web`.
---
## Architecture
<!-- ARCHITECTURE DIAGRAM PLACEHOLDER: Mermaid system diagram showing browser, Next.js on Vercel, Go API as Vercel serverless functions, Redis on Upstash, Postgres on Neon, Go indexer on GitHub Actions cron, and Soroban RPC -->
> See `ARCHITECTURE.md` for the full system diagram, data flows, schema DDL, and REST API reference.
---
## Tech stack
| Layer | Technology |
|---|---|
| API | Go 1.23, chi, pgx v5, deployed as Vercel serverless functions |
| Indexer | Go 1.23, runs as a GitHub Actions scheduled workflow (5-minute cron) |
| Database | Postgres 16, hosted on Neon |
| Cache / locks | Redis, hosted on Upstash |
| Dashboard | Next.js 15, TypeScript, Tailwind CSS, deployed on Vercel |
| XDR decoder | TypeScript package (`packages/xdr`), wraps `@stellar/stellar-sdk` |
| CLI | Go 1.23, cobra |
| Fixture contract | Rust (stable), Soroban SDK, deployed to Stellar testnet |
---
## Monorepo layout
```
sorolens/
  apps/
    api/          Go API (Vercel serverless functions)
    web/          Next.js 15 dashboard
  services/
    indexer/      Go indexer worker
  packages/
    xdr/          TypeScript XDR decoder
  cli/            Go CLI (cobra)
  contracts/
    counter/      Rust Soroban fixture contract
  docs/
    screenshots/  Screenshot placeholders
```
---
## Contributing
See [CONTRIBUTING.md](./CONTRIBUTING.md) for local setup, branch conventions, commit format, and a full walkthrough of adding a new XDR type decoder.
---
## Code of conduct
See [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md).
---
## License
MIT. See [LICENSE](./LICENSE).
