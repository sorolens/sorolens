[![CI](https://github.com/sorolens/sorolens/actions/workflows/ci.yml/badge.svg)](https://github.com/sorolens/sorolens/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/sorolens/sorolens/branch/main/graph/badge.svg)](https://codecov.io/gh/sorolens/sorolens)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/sorolens/sorolens)](https://github.com/sorolens/sorolens/releases/latest)
[![Discord](https://img.shields.io/badge/Discord-Join-5865F2?logo=discord&logoColor=white)](https://discord.gg/D9jATUezYX)
[![Telegram](https://img.shields.io/badge/Telegram-Join-26A5E4?logo=telegram&logoColor=white)](https://t.me/sorolens_community)
[![Go Report Card (API)](https://goreportcard.com/badge/github.com/sorolens/sorolens/apps/api)](https://goreportcard.com/report/github.com/sorolens/sorolens/apps/api)
[![Go Report Card (Indexer)](https://goreportcard.com/badge/github.com/sorolens/sorolens/services/indexer)](https://goreportcard.com/report/github.com/sorolens/sorolens/services/indexer)
# Sorolens
Real-time monitoring, alerting, and on-chain health checks for Soroban smart contracts on Stellar. The only observability tool in the Stellar ecosystem with a deployed Soroban **watchdog** contract for proactive contract monitoring.
---
## The problem
Soroban's public RPC retains events for 24 hours and transaction data for up to 7 days. There is no persistent index of what your contract did, what it cost, or which storage entries are close to expiring. The Stellar Lab contract explorer is excellent for point-in-time inspection but has no history, no API, and no way to know when a contract you depend on is unhealthy. Sorolens fills that gap.
---
## What it does
- **Contract event indexing**: every event your contract emits, with decoded topics and values, indexed into Postgres and queryable via REST.
- **Invocation tracing**: per-transaction CPU instructions, memory, ledger I/O bytes, and fees charged.
- **Storage tracking**: snapshot of every temporary, persistent, and instance storage entry, with TTL health.
- **Watchdog monitoring** *(new)*: the on-chain `sorolens-watchdog` Soroban contract lets any contract owner register a contract, push health status, and raise alerts. Sorolens indexes those events and materialises them into a dashboard, giving you the only proactive-monitoring path on Stellar without a centralised heartbeat service.
- **Alerting**: when persistent storage entries are within a configurable number of ledgers of expiry, or when a monitored contract goes `Unresponsive`.
- **Multi-network**: track and query testnet, mainnet, and futurenet contracts side by side, with a network selector in the dashboard header and a `?network=` filter on every list endpoint.
- **Snapshot / replay**: `GET /api/v1/contracts/:id/snapshot?ledger=N` replays a contract's storage state and last known event as of any ledger, with a ledger scrubber on the contract page for time-travel debugging.
- **Scoped API keys**: per-key permissions (`read:contracts`, `write:contracts`, `read:watchdog`, `admin:*`) enforced by route metadata, so a monitoring bot can hold a read-only watchdog key.
- **API playground**: an interactive `/playground` page to explore every endpoint, send requests, and copy them as curl.

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
## How to contribute

1. **Find an issue** – Browse the [open issues](https://github.com/sorolens/sorolens/issues) and pick one that interests you. Look for labels like `good first issue` or `help wanted`.
2. **Claim it** – Comment "I'd like to work on this" on the issue. A maintainer will assign it to you.
3. **Open a PR** – Follow the branch naming and commit conventions in [CONTRIBUTING.md](./CONTRIBUTING.md) and open a pull request against `main`.

See [CONTRIBUTING.md](./CONTRIBUTING.md) for the full walkthrough, including local setup, running tests, and the PR checklist.

Real-time chat with maintainers and other contributors: [**join the Sorolens Discord**](https://discord.gg/D9jATUezYX). Announcements, PR reviews, weekly office hours, and a live GitHub activity feed all live there. Prefer Telegram? Mirror is at [t.me/sorolens_community](https://t.me/sorolens_community); Discord is the primary hub.

---
## Architecture

```
             +-------------------+       +--------------------+
             |   Soroban RPC     | <---- |  Sorolens indexer  |
             +-------------------+       |  (Go, GH Actions)  |
                                         +--------+-----------+
                                                  |
                                                  v
     +-----------------+     +-------+     +---------------+     +-------------+
     |  Watchdog       |     |       |     |  Postgres 16  | <-- |  REST API   |
     |  (Soroban)      |==>events==>|     |  (Neon)       |     |  (Go, Vercel)|
     +-----------------+     +-------+     +---------------+     +------+------+
                                                                        |
                                                                        v
                                                                +---------------+
                                                                |  Next.js 15   |
                                                                |  dashboard    |
                                                                |  (Vercel)     |
                                                                +---------------+
```

## Screenshots

### Landing page

![Sorolens landing page](docs/screenshots/landing.png)

### Contracts list

![Contracts list](docs/screenshots/contracts.png)

### Contract detail

![Contract detail](docs/screenshots/contract-detail.png)

### Watchdog overview

![Watchdog overview](docs/screenshots/watchdog.png)

### Watchdog contract detail

![Watchdog contract detail](docs/screenshots/watchdog-contract-detail.png)

The **watchdog contract** at `contracts/watchdog/` is the piece that makes Sorolens unique: contracts you operate emit `HealthCheckEvent`, `ContractAlert`, `ContractRegistered`, and `ContractDeregistered` on-chain, the indexer picks them up along with everything else, and the dashboard renders your fleet's live health.

> See `ARCHITECTURE.md` for the full system diagram, data flows, schema DDL, and REST API reference.

### API reference

The REST API is described by an OpenAPI 3.0 spec at [`docs/openapi.yaml`](docs/openapi.yaml). Load it into Swagger UI, Redoc or Postman, or generate a client from it (the Go client in `packages/go-client` is generated this way). Run `make openapi` after changing a route: it fails if any route is undocumented, lints the spec with Redocly, and regenerates the Go client.

---
## Deployed contracts

### Testnet

| Contract | ID | Explorer |
|----------|----|----------|
| Watchdog | `CACXRL67WL5KRD6HKWGYADHEUF6RQOCODUN26UQE7MGFZEMIR7PAX6R7` | [Stellar Expert](https://stellar.expert/explorer/testnet/contract/CACXRL67WL5KRD6HKWGYADHEUF6RQOCODUN26UQE7MGFZEMIR7PAX6R7) |

Admin: `GAZ3HN2QNDKWLOI2OQEG65KBJEAUP4PROR3FJNXNDY34UH547MN4CJUI`

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
| Go client | Generated from [`docs/openapi.yaml`](docs/openapi.yaml) into `packages/go-client` |
| Fixture contract | Rust (stable), Soroban SDK, deployed to Stellar testnet |
| Watchdog contract | Rust (stable), Soroban SDK, on-chain health tracking (`contracts/watchdog`) |
---
## Monorepo layout
```
sorolens/
  apps/
    api/          Go API (Vercel serverless functions)
    web/          Next.js 15 dashboard (landing page + /contracts + /watchdog + /playground)
  services/
    indexer/      Go indexer worker (+ internal/watchdog event classifier)
  packages/
    xdr/          TypeScript XDR decoder
    ui/           Shared React UI primitives
    go-client/    Auto-generated Go API client (from docs/openapi.yaml)
  cli/            Go CLI (cobra)
  contracts/
    counter/      Rust Soroban fixture contract
    watchdog/     Rust Soroban watchdog contract (on-chain health tracking)
  docs/
    screenshots/  Screenshot placeholders
```

### The watchdog vertical

| Layer | Location |
|---|---|
| On-chain contract | `contracts/watchdog/src/lib.rs` (`register_contract`, `report_status`, `report_alert`, `get_all_monitored`, ...) |
| Deploy script | `contracts/watchdog/scripts/deploy-testnet.sh` |
| Event classifier | `services/indexer/internal/watchdog/classifier.go` |
| Postgres schema | `apps/api/internal/db/migrations/000002_watchdog.up.sql` (`monitored_contracts`, `health_checks`, `contract_alerts`) |
| Store methods | `apps/api/internal/store/watchdog.go` |
| REST endpoints | `GET /api/v1/watchdog/{stats,contracts,alerts,contracts/{id}/{health,alerts}}` |
| Dashboard pages | `apps/web/app/(app)/watchdog/*` |
---
## Contributing
### See [CONTRIBUTING.md](./CONTRIBUTING.md) for local setup, branch conventions, commit format, and a full walkthrough of adding a new XDR type decoder.
---
## Code of conduct
### See [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md).
---

## Contributors

Thanks to everyone who has contributed to Sorolens!

[![Contributors](https://contrib.rocks/image?repo=sorolens/sorolens)](https://github.com/sorolens/sorolens/graphs/contributors)


## License
MIT. See [LICENSE](./LICENSE).
