# Getting started

Sorolens indexes Soroban contract events, invocations and storage state, then
serves them through a REST API and this dashboard. The quickest path from zero
to a populated dashboard takes about five minutes.

## 1. Run the stack locally

Postgres 16 and Redis come from Docker. The one-shot `migrate` service applies
the database schema on first start.

```bash
docker compose up -d
make api
```

The API listens on http://localhost:8080. Start the dashboard in a second
terminal:

```bash
cd apps/web
cp .env.local.example .env.local
pnpm install
pnpm dev
```

Set `NEXT_PUBLIC_API_URL=http://localhost:8080` in `.env.local` so the
dashboard talks to your local API.

## 2. Track a contract

Every dashboard view is built from contracts you track. Use the CLI, the
dashboard's contract wizard, or the REST API.

```bash
sorolens contracts add CAVRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5
```

After the next indexer tick, the contract page shows events, invocations,
storage entries and resource usage.

## 3. Explore the REST API

The API reference is generated from `docs/openapi.yaml` and is also browsable
in the in-product [API playground](/playground).

- OpenAPI source: https://github.com/sorolens/sorolens/blob/main/docs/openapi.yaml
- Go client: https://github.com/sorolens/sorolens/tree/main/packages/go-client
- TypeScript XDR decoder: https://github.com/sorolens/sorolens/tree/main/packages/xdr
- Python SDK: https://github.com/sorolens/sorolens/tree/main/packages/python-sdk

## 4. Next steps

- Read [Concepts](/docs/concepts) for the data model and indexing lifecycle.
- Configure alerts and watchdog monitoring from the dashboard.
- Read the [deployment guide](https://github.com/sorolens/sorolens/blob/main/docs/deployment.md)
  before running Sorolens in production.
