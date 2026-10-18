# SoroLens Deployment Guide

This guide covers production deployment strategies for SoroLens across managed cloud providers (**Neon**, **Upstash**, **Vercel**) and **self-hosted Docker** environments, including full configuration for **Stellar Mainnet** as outlined in [`ROADMAP.md`](../ROADMAP.md).

---

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Prerequisites](#prerequisites)
- [1. Managed Database: Neon (PostgreSQL 16)](#1-managed-database-neon-postgresql-16)
  - [Connection Pooling: Pooled vs Direct](#connection-pooling-pooled-vs-direct)
  - [Applying Migrations](#applying-migrations)
- [2. Managed Cache: Upstash (Redis)](#2-managed-cache-upstash-redis)
- [3. Frontend and API: Vercel](#3-frontend-and-api-vercel)
  - [Deploying the Dashboard (`apps/web`)](#deploying-the-dashboard-appsweb)
  - [Deploying the API (`apps/api`) as Serverless Functions](#deploying-the-api-appsapi-as-serverless-functions)
- [4. Self-Hosting with Docker & Docker Compose](#4-self-hosting-with-docker--docker-compose)
  - [Production Stack Topology](#production-stack-topology)
  - [Building and Running the Services](#building-and-running-the-services)
  - [Health Verification](#health-verification)
- [5. Mainnet Configuration](#5-mainnet-configuration)
  - [Network Passphrase and RPC Selection](#network-passphrase-and-rpc-selection)
  - [Mainnet Horizon and Watchdog Status](#mainnet-horizon-and-watchdog-status)
- [6. Environment Variable Reference](#6-environment-variable-reference)
  - [API and Global Variables](#api-and-global-variables)
  - [Indexer Variables](#indexer-variables)
  - [Web Dashboard Variables](#web-dashboard-variables)
  - [CLI Variables](#cli-variables)
- [7. Troubleshooting](#7-troubleshooting)

---

## Architecture Overview

A production SoroLens deployment consists of four distinct operational layers:

```
                           ┌─────────────────────────────────────────┐
                           │            Stellar Mainnet              │
                           │   Soroban RPC Node (e.g. QuickNode)     │
                           └───────────────▲─────────────────────────┘
                                           │ JSON-RPC
                                           │
┌──────────────────────────┐      ┌────────┴────────┐      ┌─────────────────────────┐
│       Next.js Web        │ HTTP │   Go REST API   │ SQL  │    Neon PostgreSQL 16   │
│    Dashboard (Vercel)    ├─────►│ (Docker/Vercel) ├─────►│  (Pooled & Direct DSN)  │
└──────────────────────────┘      └────────┬────────┘      └─────────────────────────┘
                                           │                           ▲
                                     Redis │                     SQL   │
                                           ▼                           │
                                  ┌─────────────────┐      ┌───────────┴─────────────┐
                                  │  Upstash Redis  │      │     SoroLens Indexer    │
                                  │ (Cache & Locks) │◄─────┤ (Continuous/Cron Worker)│
                                  └─────────────────┘      └─────────────────────────┘
```

1. **REST API (`apps/api`)**: Go HTTP service providing contract observability, event streams, analytics, and health scoring.
2. **Indexer (`services/indexer`)**: Background worker that polls Soroban RPC, processes contract events, traces invocations, and updates contract storage snapshots.
3. **Dashboard (`apps/web`)**: Next.js 15 frontend with Tailwind CSS for visualizing contracts, invocations, TTL health, and watchdog telemetry.
4. **Data Tier**: PostgreSQL 16 (relational events & state) and Redis 7 (response cache, rate-limiting, and indexer run locks).

---

## Prerequisites

Before starting, ensure you have:
- [Docker Engine](https://docs.docker.com/engine/install/) 24+ and [Docker Compose](https://docs.docker.com/compose/) v2.20+ (for self-hosting).
- [Go](https://go.dev/) 1.23+ and [Node.js](https://nodejs.org/) 22+ with [pnpm](https://pnpm.io/) 9+ (for local builds and migration CLI).
- Accounts on [Neon](https://neon.tech), [Upstash](https://upstash.com), and [Vercel](https://vercel.com) (if using managed services).
- A Stellar Soroban RPC endpoint for your target network (Mainnet or Testnet).

---

## 1. Managed Database: Neon (PostgreSQL 16)

SoroLens requires PostgreSQL 16+. Neon provides serverless Postgres with instant branching and built-in connection pooling.

### Project Setup

1. Log into the [Neon Console](https://console.neon.tech/) and click **New Project**.
2. Name the project `sorolens-prod` and select **PostgreSQL 16**.
3. Choose the cloud region closest to your API and indexer deployment (e.g. `aws-us-east-1` or `aws-eu-central-1`).
4. Save the generated database credentials.

### Connection Pooling: Pooled vs Direct

Neon provides two distinct connection strings in the dashboard:

```
# Pooled (uses Neon's PgBouncer in transaction mode)
postgres://user:pass@ep-cool-cloud-123456-pooler.us-east-1.aws.neon.tech/sorolens?sslmode=require

# Direct (unpooled, connects directly to Postgres compute node)
postgres://user:pass@ep-cool-cloud-123456.us-east-1.aws.neon.tech/sorolens?sslmode=require
```

#### When to use which connection string:

| Connection Type | Environment Variable | Usage in SoroLens | Reason / Caveats |
| :--- | :--- | :--- | :--- |
| **Pooled DSN** | `DATABASE_URL` | Runtime Go API (`apps/api`) and Indexer (`services/indexer`) | Binds connections through PgBouncer in transaction mode, allowing thousands of concurrent client queries without exhausting Postgres process limits. |
| **Direct DSN** | `DIRECT_DATABASE_URL` | Schema migrations (`make migrate-up`, CI, and `migrate` service) | **Crucial**: Transaction-mode poolers do not support DDL statements that alter search paths, lock tables across sessions, or rely on prepared statements. Schema migrations **must** bypass the pooler. |

> [!IMPORTANT]
> **Advisory Lock Caveat**: Neon's pooled connection string runs in PgBouncer transaction mode. The sharded coordinator's leader election (`pg_try_advisory_lock`) requires session-scoped connections. If running a sharded coordinator on Neon, point the coordinator process at `DIRECT_DATABASE_URL` rather than the pooled endpoint.

### Applying Migrations

The repository maintains an ordered sequence of migrations under `apps/api/internal/db/migrations/*.up.sql`.

To apply all migrations to your Neon database from your local machine:

```bash
# Set DIRECT_DATABASE_URL to your unpooled Neon connection string
export DIRECT_DATABASE_URL="postgres://sorolens:PASSWORD@ep-your-endpoint.us-east-1.aws.neon.tech/sorolens?sslmode=require"

# Apply migrations sequentially using psql
for f in $(ls apps/api/internal/db/migrations/*.up.sql | sort); do
  echo "Applying $f..."
  psql "$DIRECT_DATABASE_URL" -f "$f"
done
```

Alternatively, use the Docker-based `golang-migrate` target in the Makefile:

```bash
# Baseline a new database or run up migrations
make migrate-up DB_URL_LOCAL="$DIRECT_DATABASE_URL"
```

Verify that the schema was created:
```bash
psql "$DIRECT_DATABASE_URL" -c "\dt"
```
You should see tables: `contracts`, `events`, `invocations`, `contract_storage`, `failed_events`, `indexer_cursors`, `api_keys`, `monitored_contracts`, etc.

---

## 2. Managed Cache: Upstash (Redis)

SoroLens uses Redis 7 for:
1. API response caching (`GET /api/v1/contracts`, `GET /api/v1/watchdog/stats`).
2. Per-client sliding window rate limiting.
3. Indexer global run locking (preventing overlapping runs).

### Setup

1. In the [Upstash Console](https://console.upstash.com/), click **Create Database**.
2. Name the database `sorolens-cache` and select the same region as your Neon database.
3. Under **Security**, ensure TLS is enabled.
4. In the database dashboard, copy the standard Redis connection string:
   ```
   rediss://default:YOUR_PASSWORD@your-endpoint.upstash.io:6379
   ```

### Application Connection

Set the `REDIS_URL` environment variable:
```bash
REDIS_URL="rediss://default:YOUR_PASSWORD@your-endpoint.upstash.io:6379"
```

> [!NOTE]
> Ensure the scheme is `rediss://` (with double `s`), which instructs the Go `redis.ParseURL` parser to establish an encrypted TLS connection.

---

## 3. Frontend and API: Vercel

### Deploying the Dashboard (`apps/web`)

The dashboard is built on Next.js 15 and deploys to Vercel with zero extra infrastructure.

1. Push your repository fork or clone to GitHub/GitLab.
2. In the [Vercel Dashboard](https://vercel.com/new), import the repository.
3. Configure the project:
   - **Framework Preset**: Next.js
   - **Root Directory**: `apps/web`
   - **Build Command**: `pnpm build`
   - **Output Directory**: `.next`
4. Configure Environment Variables in Vercel:
   - `NEXT_PUBLIC_API_URL`: The public URL of your deployed SoroLens API (e.g. `https://api.sorolens.example.com` or your Docker/Railway URL).
   - `NEXT_PUBLIC_SITE_URL`: The public domain of the dashboard (e.g. `https://sorolens.example.com`).
   - `NEXT_PUBLIC_PWA_ENABLED`: Set to `true` if Progressive Web App service workers are desired.
5. Click **Deploy**.

---

### Deploying the API (`apps/api`) as Serverless Functions

The Go API includes a Vercel serverless adapter in `apps/api/api/index.go`.

#### Vercel Serverless Considerations

> [!WARNING]
> **Serverless Connection Limits & SSE Limitations**:
> - Vercel Serverless Functions have strict maximum execution durations (15s on Hobby, 60s/300s on Pro).
> - Long-lived SSE connections (`/api/v1/stream/events`) will terminate when the function reaches its execution timeout; clients fall back to polling.
> - The background Indexer cannot run as a Vercel Serverless Function and must run as a scheduled GitHub Action, container, or dedicated worker.

To deploy the API to Vercel:
1. Create a separate Vercel project pointing to the repository root.
2. In the project settings:
   - **Root Directory**: `.`
3. Set the required environment variables:
   - `DATABASE_URL`: Your pooled Neon connection string (`...-pooler...sslmode=require`).
   - `REDIS_URL`: Your Upstash connection string (`rediss://...`).
   - `CORS_ALLOWED_ORIGINS`: Your dashboard's Vercel origin (e.g. `https://sorolens.vercel.app`).
   - `STELLAR_NETWORK`: `mainnet` or `testnet`.
   - `SOROBAN_RPC_URL`: Mainnet or Testnet RPC URL.
   - `IDENTITY_SIGNING_KEY`: 64-character random hex string for HMAC session signing (`openssl rand -hex 32`).

---

## 4. Self-Hosting with Docker & Docker Compose

For complete control without serverless execution timeouts or cold starts, self-host the entire stack using Docker Compose.

### Production Stack Topology

The repository provides [`docker-compose.prod.yml`](../docker-compose.prod.yml), which orchestrates:
1. `postgres`: PostgreSQL 16 on port 5432 with persistent volume `sorolens_postgres_data`.
2. `redis`: Redis 7 on port 6379 with persistent volume `sorolens_redis_data` and AOF persistence enabled.
3. `migrate`: One-shot migration container that applies all `apps/api/internal/db/migrations/*.up.sql` scripts and exits before the API boots.
4. `api`: Production-optimized Go binary built using [`apps/api/Dockerfile`](../apps/api/Dockerfile) (~20MB distroless non-root image).

```
                      docker-compose.prod.yml
 ┌───────────────────────────────────────────────────────────────┐
 │                                                               │
 │   ┌──────────────┐     ┌──────────────┐                       │
 │   │   postgres   │◄────┤   migrate    │ (one-shot exit)       │
 │   │  (Port 5432) │     └──────────────┘                       │
 │   └───────▲──────┘                                            │
 │           │                                                   │
 │           ├────────────────────┐                              │
 │   ┌───────┴──────┐     ┌───────┴──────┐      ┌─────────────┐  │
 │   │    redis     │◄────┤     api      │◄─────┤   ingress   │  │
 │   │  (Port 6379) │     │  (Port 8080) │      │  (Port 80)  │  │
 │   └──────────────┘     └──────────────┘      └─────────────┘  │
 └───────────────────────────────────────────────────────────────┘
```

---

### Building and Running the Services

#### Step 1: Create a Production Environment File

Create `.env.prod` based on [`.env.example`](../.env.example):

```bash
# --- Infrastructure ---
POSTGRES_PASSWORD=generate_a_secure_postgres_password_here
DATABASE_URL=postgres://sorolens:generate_a_secure_postgres_password_here@postgres:5432/sorolens?sslmode=disable
DIRECT_DATABASE_URL=postgres://sorolens:generate_a_secure_postgres_password_here@postgres:5432/sorolens?sslmode=disable
REDIS_URL=redis://redis:6379/0

# --- Stellar Network ---
STELLAR_NETWORK=mainnet
SOROBAN_RPC_URL=https://soroban-mainnet.stellar.org:443
SOROBAN_RPC_URL_MAINNET=https://soroban-mainnet.stellar.org:443

# --- API Configuration ---
PORT=8080
LOG_LEVEL=info
CORS_ALLOWED_ORIGINS=https://dashboard.yourdomain.com
IDENTITY_SIGNING_KEY=generate_64_char_hex_secret_with_openssl_rand_hex_32
API_REQUEST_TIMEOUT=30s
API_STREAM_TIMEOUT=5m

# --- Optional Admin Bootstrap ---
INITIAL_ADMIN_GITHUB_ID=your_github_numeric_id
```

#### Step 2: Build Images from Monorepo Root

Build the production API image using the multi-stage [`apps/api/Dockerfile`](../apps/api/Dockerfile):

```bash
docker build \
  --build-arg VERSION=$(git describe --tags --always 2>/dev/null || echo "v0.3.0") \
  --build-arg GIT_SHA=$(git rev-parse --short HEAD 2>/dev/null || echo "prod") \
  --build-arg BUILT_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
  -t sorolens/api:latest \
  -f apps/api/Dockerfile \
  .
```

Build the Indexer worker image using [`services/indexer/Dockerfile`](../services/indexer/Dockerfile):

```bash
docker build \
  -t sorolens/indexer:latest \
  -f services/indexer/Dockerfile \
  .
```

#### Step 3: Launch Database, Redis, and API

```bash
docker compose --env-file .env.prod -f docker-compose.prod.yml up -d
```

Monitor container startup and migration progress:
```bash
docker compose -f docker-compose.prod.yml logs -f migrate
docker compose -f docker-compose.prod.yml logs -f api
```

#### Step 4: Run the Indexer Worker

You can run the indexer in persistent continuous mode as a background service attached to the same Docker network:

```bash
docker run -d \
  --name sorolens-indexer \
  --restart unless-stopped \
  --network sorolens_net \
  --env-file .env.prod \
  sorolens/indexer:latest \
  -mode continuous \
  -poll-interval 5m \
  -metrics-addr :9100
```

---

### Health Verification

Verify that all endpoints respond properly:

```bash
# 1. API Liveness Probe
curl -s http://localhost:8080/health | jq .
# Returns: {"status":"ok"}

# 2. API Readiness Probe (verifies Postgres and Redis connectivity)
curl -s http://localhost:8080/readyz | jq .
# Returns: {"status":"ok"}

# 3. Version and Build Metadata
curl -s http://localhost:8080/api/version | jq .

# 4. Prometheus Metrics Scrape
curl -s http://localhost:8080/metrics | grep -E "sorolens_http|sorolens_db_pool"

# 5. Indexer Metrics Scrape
curl -s http://localhost:9100/metrics | grep sorolens_indexer
```

---

## 5. Mainnet Configuration

The Stellar mainnet operates with distinct network identifiers, RPC requirements, and ledger velocity compared to testnet.

### Network Passphrase and RPC Selection

1. **Stellar Network Variable**:
   ```ini
   STELLAR_NETWORK=mainnet
   ```
   This ensures cryptographic signature verification and network identifiers use the official Stellar Mainnet passphrase:
   `Public Global Stellar Network ; September 2015`.

2. **Soroban RPC Endpoint**:
   The Stellar Development Foundation (SDF) does not provide an open public RPC endpoint for Mainnet without rate limits. Operators must use a dedicated Soroban RPC provider (e.g. [QuickNode](https://www.quicknode.com/), [NowNodes](https://nownodes.io/), [Blockdaemon](https://www.blockdaemon.com/)) or run a self-hosted `stellar-core` + Soroban RPC node.

   Configure the network-specific URL in your environment:
   ```ini
   SOROBAN_RPC_URL_MAINNET="https://your-mainnet-rpc-provider.com/v1/..."
   SOROBAN_RPC_URL="https://your-mainnet-rpc-provider.com/v1/..."
   SOROBAN_RPC_TIMEOUT=30s
   SOROBAN_RPC_MAX_RETRIES=5
   ```

3. **Ledger Window and History Retention**:
   Mainnet generates high event volume. Soroban RPC nodes typically retain historical events for **24 hours to 7 days**.
   ```ini
   # 120960 ledgers at ~5s per ledger is approximately 7 days:
   INDEXER_LEDGER_WINDOW=120960
   INDEXER_POLL_INTERVAL=1m
   ```

### Mainnet Horizon and Watchdog Status

- **Watchdog Contract**:
  - Testnet Watchdog: `CACXRL67WL5KRD6HKWGYADHEUF6RQOCODUN26UQE7MGFZEMIR7PAX6R7`
  - Mainnet Watchdog: Marked *TBA — not deployed yet* in [`README.md`](../README.md#mainnet). Leave `WATCHDOG_CONTRACT_ID=` empty when operating exclusively on Mainnet until the official on-chain contract address is published.
- **Contract Registration**:
  Use the CLI to track a mainnet contract once deployed:
  ```bash
  sorolens track CB64... --network mainnet --api-url http://localhost:8080
  ```

---

## 6. Environment Variable Reference

This reference covers all environment variables parsed across `apps/api`, `services/indexer`, `apps/web`, and `cli/`.

### API and Global Variables

| Variable | Component | Required? | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `DATABASE_URL` | API / Indexer | **Yes** | — | PostgreSQL connection string (pooled for API, unpooled for direct). |
| `DIRECT_DATABASE_URL` | API / Migrate | Optional | — | Non-pooled connection string used for schema migrations. |
| `REDIS_URL` | API / Indexer | **Yes** | — | Redis connection string (`redis://...` or `rediss://...` for TLS). |
| `PORT` | API | Optional | `8080` | HTTP listen port for the API server. |
| `METRICS_PORT` | API | Optional | — | Dedicated private admin port for `GET /metrics`. |
| `LOG_LEVEL` | API / Indexer | Optional | `info` | Log verbosity: `debug`, `info`, `warn`, `error`. |
| `STELLAR_NETWORK` | API / CLI | Optional | `testnet` | Target Stellar network: `mainnet`, `testnet`, or `futurenet`. |
| `SOROBAN_RPC_URL` | API / Indexer | Optional | `https://soroban-testnet.stellar.org` | Default fallback Soroban JSON-RPC endpoint. |
| `CORS_ALLOWED_ORIGINS` | API | Optional | `*` | Comma-separated list of allowed CORS origins for dashboard access. |
| `IDENTITY_SIGNING_KEY` | API | Optional | *ephemeral hex* | 32-byte secret for signing HMAC workspace session tokens (`sls_...`). |
| `IDENTITY_SESSION_TTL` | API | Optional | `720h` | Duration that session bearer tokens remain valid. |
| `TRUSTED_IDENTITY_PROXY_SECRET`| API | Optional | — | Shared secret for verifying signed reverse proxy identity headers. |
| `REPORT_SIGNING_KEY` | API | Optional | — | HMAC secret used to cryptographically sign SLA contract reports. |
| `SLACK_SIGNING_SECRET` | API | Optional | — | Secret to verify incoming slash commands from Slack. |
| `API_CACHE_TTL` | API | Optional | `30s` | Lifetime of cached responses in Redis (`0` disables caching). |
| `API_REQUEST_TIMEOUT` | API | Optional | `30s` | Wall-clock limit for handling API requests before 503 timeout. |
| `API_STREAM_TIMEOUT` | API | Optional | `5m` | Lifetime of SSE streaming connections before reconnect. |
| `REQUEST_MAX_BODY_BYTES` | API | Optional | `1048576` (1MB) | Maximum allowed size of incoming HTTP request bodies. |
| `INITIAL_ADMIN_GITHUB_ID` | API | Optional | — | GitHub numeric ID to bootstrap with the `admin` role on boot. |
| `SENTRY_DSN` | API / Indexer | Optional | — | Sentry error tracking DSN. |
| `SENTRY_ENVIRONMENT` | API / Indexer | Optional | `production` | Deployment environment tag for Sentry. |
| `COLD_STORAGE_BUCKET` | API | Optional | — | S3 bucket name for archival storage (disabled if empty). |
| `COLD_STORAGE_ENDPOINT` | API | Optional | — | Custom S3 endpoint for MinIO / Backblaze / Cloudflare R2. |
| `COLD_STORAGE_REGION` | API | Optional | `us-east-1` | S3 signing region. |
| `COLD_STORAGE_ACCESS_KEY_ID` | API | Optional | — | S3 access key ID. |
| `COLD_STORAGE_SECRET_ACCESS_KEY` | API | Optional | — | S3 secret access key. |
| `COLD_STORAGE_THRESHOLD_DAYS` | API | Optional | `90` | Age in days before events are archived out of Postgres. |
| `GRAPHQL_COMPLEXITY_LIMIT` | API | Optional | `5000` | Maximum complexity score permitted for GraphQL queries. |
| `GRAPHQL_PERSISTED_ONLY` | API | Optional | `false` | When true, rejects ad-hoc GraphQL queries. |
| `VERIFY_BUILD_COMMAND` | API | Optional | `stellar contract build` | Command used for source code verification builds. |
| `VERIFY_TIMEOUT` | API | Optional | `10m` | Execution timeout for contract verification builds. |
| `VERIFY_WORKSPACE_DIR` | API | Optional | *temp dir* | Host directory for verification workspaces. |
| `SMTP_ADDR` | API | Optional | — | Host and port for digest email notifications (e.g. `smtp.mailgun.org:587`). |
| `SMTP_FROM` | API | Optional | — | From email address for report digests. |
| `SMTP_USER` / `SMTP_PASS` | API | Optional | — | Credentials for SMTP authentication. |
| `OTEL_EXPORTER_OTLP_ENDPOINT`| API / Indexer | Optional | — | OpenTelemetry collector endpoint. |

---

### Indexer Variables

| Variable | Component | Required? | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `SOROBAN_RPC_URL_TESTNET` | Indexer | Optional | — | Dedicated RPC endpoint for `testnet` contracts. |
| `SOROBAN_RPC_URL_MAINNET` | Indexer | Optional | — | Dedicated RPC endpoint for `mainnet` contracts. |
| `SOROBAN_RPC_URL_FUTURENET` | Indexer | Optional | — | Dedicated RPC endpoint for `futurenet` contracts. |
| `SOROBAN_RPC_TIMEOUT` | Indexer | Optional | `30s` | Timeout for individual Soroban RPC requests. |
| `SOROBAN_RPC_MAX_RETRIES` | Indexer | Optional | `3` | Maximum backoff retries on RPC HTTP 429 or 5xx. |
| `INDEXER_WORKERS` | Indexer | Optional | `GOMAXPROCS` | Number of concurrent contract indexing worker routines. |
| `INDEXER_POLL_INTERVAL` | Indexer | Optional | `5m` | Sleep interval between passes in continuous mode. |
| `INDEXER_LEDGER_WINDOW` | Indexer | Optional | `120960` | Backfill window size in ledgers (~7 days). |
| `INDEXER_MAX_DURATION` | Indexer | Optional | `270s` | Maximum execution time per pass in `once` mode. |
| `INDEXER_METRICS_ADDR` | Indexer | Optional | `:9100` | Address for Prometheus `/metrics` HTTP server. |
| `INDEXER_ROLE` | Indexer | Optional | `all` | Sharding role: `all`, `coordinator`, or `worker`. |
| `INDEXER_SHARD_COUNT` | Indexer | Optional | `64` | Total number of virtual shards for contracts. |
| `INDEXER_WORKER_ID` | Indexer | Optional | — | Process identifier (required for coordinator/worker). |
| `INDEXER_SHARD_STORE` | Indexer | Optional | — | Sharding backend: set to `memory` on single node. |
| `INDEXER_STORAGE_EXPIRY_ALERT_ENABLED` | Indexer | Optional | `true` | Enable automated storage TTL alerts. |
| `INDEXER_STORAGE_EXPIRY_THRESHOLD_LEDGERS` | Indexer | Optional | `5000` | Ledgers until expiration to trigger warning (~7 days). |
| `INDEXER_STORAGE_EXPIRY_MAX_ENTRIES` | Indexer | Optional | `200` | Max near-expiry entries scanned per contract. |
| `WATCHDOG_ENABLED` | Indexer | Optional | `false` | Enable watchdog contract event classification. |
| `WATCHDOG_CONTRACT_ID` | Indexer / API | Optional | — | Contract address of deployed watchdog contract. |

---

### Web Dashboard Variables

| Variable | Component | Required? | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `NEXT_PUBLIC_API_URL` | Web | Optional | `http://localhost:8080` | Browser URL pointing to the SoroLens REST API. |
| `NEXT_PUBLIC_SITE_URL` | Web | Optional | `http://localhost:3000` | Public base domain of the web application. |
| `NEXT_PUBLIC_PWA_ENABLED` | Web | Optional | `false` | Enable service workers for mobile Progressive Web App. |
| `NEXT_PUBLIC_SENTRY_DSN` | Web | Optional | — | Sentry DSN for frontend exception tracking. |
| `NEXT_PUBLIC_PLAUSIBLE_DOMAIN`| Web | Optional | — | Domain name for privacy-preserving Plausible Analytics. |
| `INDEXER_METRICS_URL` | Web | Optional | — | Backend URL for `/status` page to scrape indexer metrics. |
| `VAPID_PUBLIC_KEY` | Web | Optional | — | Web Push public key for browser push notifications. |
| `VAPID_PRIVATE_KEY` | Web | Optional | — | Web Push private key (server-side only). |
| `VAPID_SUBJECT` | Web | Optional | `mailto:admin@sorolens.dev` | Subject mailto/URI for VAPID JWT tokens. |
| `PUSH_INTERNAL_SECRET` | Web | Optional | — | Secret token guarding internal push subscription API. |

---

### CLI Variables

| Variable | Component | Required? | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `SOROLENS_API_URL` | CLI | Optional | `http://localhost:8080` | Base API URL targeted by the CLI. |
| `SOROLENS_NETWORK` | CLI | Optional | `testnet` | Default network flag (`testnet`, `mainnet`). |
| `SOROLENS_OUTPUT` | CLI | Optional | `table` | Output format: `table` or `json`. |
| `NO_COLOR` | CLI | Optional | — | Suppress ANSI color escapes in terminal output. |
| `SOROLENS_WATCHDOG_CONTRACT_ID` | CLI | Optional | — | Overrides watchdog contract ID for registration. |

---

## 7. Troubleshooting

### Problem: Database Connection Pool Exhaustion on Neon
- **Symptom**: API logs show `failed to acquire connection from pool: context deadline exceeded`. Metric `sorolens_db_pool_empty_acquires_total` spikes.
- **Cause**: The API is configured with an unpooled direct connection string or `max_connections` ceiling exceeds Neon compute capacity.
- **Fix**:
  1. Ensure `DATABASE_URL` uses the pooled endpoint containing `-pooler.` in the hostname.
  2. Append connection limits to the DSN:
     `DATABASE_URL="postgres://user:pass@ep-pooler.neon.tech/sorolens?sslmode=require&pool_max_conns=20"`

---

### Problem: Migration Failures on Neon (`prepared statement already exists` or DDL Lock)
- **Symptom**: `make migrate-up` fails with `syntax error` or PgBouncer transaction pooling errors.
- **Cause**: Schema migrations were executed against the pooled connection string (`-pooler.`) instead of the direct endpoint.
- **Fix**: Point your migration runner (`DIRECT_DATABASE_URL`) directly at the unpooled compute endpoint (without `-pooler`).

---

### Problem: Upstash Redis Connection Refused or SSL Error
- **Symptom**: `x509: certificate signed by unknown authority` or `EOF` when connecting to Redis.
- **Cause**: Upstash requires TLS for external connections, but the DSN used `redis://` instead of `rediss://`.
- **Fix**: Update `REDIS_URL` to begin with `rediss://` to activate TLS:
  ```ini
  REDIS_URL="rediss://default:PASSWORD@your-db.upstash.io:6379"
  ```

---

### Problem: CORS Errors on Vercel Dashboard
- **Symptom**: Browser console reports: `Access to fetch at 'https://api.yourdomain.com' from origin 'https://sorolens.vercel.app' has been blocked by CORS policy`.
- **Cause**: The API's `CORS_ALLOWED_ORIGINS` variable does not include the Vercel domain.
- **Fix**: Set `CORS_ALLOWED_ORIGINS` on the API container or serverless environment:
  ```ini
  CORS_ALLOWED_ORIGINS="https://sorolens.vercel.app,https://sorolens.yourdomain.com"
  ```

---

### Problem: Indexer HTTP 429 "Too Many Requests" on Mainnet
- **Symptom**: Indexer logs show `request failed with status 429 Too Many Requests`.
- **Cause**: Public RPC nodes enforce rate limits (typically 10–20 requests/sec).
- **Fix**:
  1. Increase backoff retries and timeout:
     ```ini
     SOROBAN_RPC_MAX_RETRIES=5
     SOROBAN_RPC_TIMEOUT=45s
     ```
  2. Switch to a dedicated commercial provider endpoint (QuickNode, NowNodes, Ankr).
  3. Reduce concurrent workers:
     ```ini
     INDEXER_WORKERS=2
     ```
