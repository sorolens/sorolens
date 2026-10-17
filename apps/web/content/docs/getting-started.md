# Getting started with Sorolens

Sorolens indexes Soroban smart-contract events, invocations, storage TTLs, and
watchdog health checks. Explore that data in the dashboard or query it through
the REST API.

## Run Sorolens locally

You will need Docker Compose, Go, Node.js 22, and pnpm. From a clone of the
repository, start the local Postgres and Redis services, then run the API and
web app in separate terminals:

```bash
docker compose up -d
make api
```

```bash
cd apps/web
NEXT_PUBLIC_API_URL=http://localhost:8080 pnpm dev
```

The dashboard is available at `http://localhost:3000`. The `make api` target
uses the local database and Redis settings documented in the repository's
Makefile. To work against the hosted service instead, set
`NEXT_PUBLIC_API_URL=https://api.sorolens.xyz`.

The API displays data already indexed in PostgreSQL. To ingest live network
data locally, configure `DATABASE_URL` for the same database and
`SOROBAN_RPC_URL` for the Soroban network, then run the indexer from
`services/indexer` with `go run . -mode continuous`. See the
[indexer configuration guide](https://github.com/sorolens/sorolens/blob/main/docs/indexer.md)
for the available settings.

## Track your first contract

Install the Sorolens CLI and register a Soroban contract on the network where it
is deployed:

```bash
go install github.com/sorolens/sorolens/cli@latest
sorolens track CONTRACT_ID --network testnet
```

You can also use the **Track a contract** action on the dashboard. The indexer
then fills in event and invocation history from the configured Soroban RPC.
Open the contract page to inspect decoded events, resource usage, and storage
TTL health.

## Core concepts

- **Contracts** are tracked by their Soroban contract ID and network.
- **Events** are indexed with ledger, transaction, decoded topics, and values.
- **Invocations** include CPU instructions, memory, ledger I/O, and resource
  fees. Daily resource trends are aggregated by the API for the dashboard.
- **Storage** shows the latest indexed entries and their expiry status.
- **Watchdog** uses the Sorolens on-chain contract to publish health checks and
  alerts for monitored contracts.
- **Reports** summarize contract health and uptime history.

Visit the [API Playground](/playground), use
[contract comparison](/compare), or review [watchdog reports](/watchdog/reports).

## API reference and SDKs

The [OpenAPI specification](https://github.com/sorolens/sorolens/blob/main/docs/openapi.yaml)
describes the REST endpoints. For example, list tracked contracts with:

```bash
curl --fail-with-body 'https://api.sorolens.xyz/api/v1/contracts?limit=10'
```

Generated and maintained clients are available for the
[Go client](https://github.com/sorolens/sorolens/tree/main/packages/go-client),
[Python SDK](https://github.com/sorolens/sorolens/tree/main/packages/python-sdk),
[Rust SDK](https://github.com/sorolens/sorolens/tree/main/packages/rust-sdk),
and the [Sorolens CLI](https://github.com/sorolens/sorolens/tree/main/cli).

For deeper deployment guidance, see the
[self-hosting guide](https://github.com/sorolens/sorolens/blob/main/docs/self-hosting/aws.md).
Questions? Join the [Discord](https://discord.gg/D9jATUezYX) or
[Telegram](https://t.me/sorolens_community).
