# Getting started

Run Sorolens on your machine, track a contract and see its events in the
dashboard.

## Prerequisites

- Docker and Docker Compose
- Go 1.26+
- Node 22+ and pnpm 9+

## 1. Start local dependencies

```bash
docker compose up -d
```

This starts Postgres 16 on `localhost:5432` and Redis on `localhost:6379`. The
one-shot `migrate` service applies the schema on first start
(`docker compose logs -f migrate`). To reset to an empty database, run
`docker compose down -v` and start again.

## 2. Track a contract and run the indexer

```bash
# Register a contract with the local API
go run ./cli track CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC --network testnet
# Run one indexer cycle
go run ./services/indexer --mode once
```

The indexer needs `DATABASE_URL` and a Soroban RPC endpoint
(`SOROBAN_RPC_URL`, or `SOROBAN_RPC_URL_<NETWORK>` per network). Every setting
is in [Indexer configuration](indexer.md).

## 3. Open the dashboard

```bash
cd apps/web
pnpm dev
```

The dashboard is at <http://localhost:3000>.

## 4. Optional: local metrics stack

```bash
docker compose --profile observability up -d
./scripts/observability-smoke.sh
```

Prometheus (<http://localhost:9090>) and Grafana (<http://localhost:3001>) start
with the Sorolens Overview dashboard and alert rules provisioned. See
[Local observability stack](observability.md).

## Next steps

- Write your own alerts in the [rule language](rule-language.md).
- Create an API key: [Authentication and API keys](authentication.md).
- Deploy it: [Deployment guide](deployment.md).
