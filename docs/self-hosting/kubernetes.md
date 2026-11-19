# Self-hosting Sorolens on Kubernetes (Helm)

The chart in [`deploy/helm/sorolens`](../../deploy/helm/sorolens) runs the
Sorolens API, indexer and web dashboard in a cluster you already have. It
does not run PostgreSQL or Redis: point it at a managed database and cache, or
at ones your platform team runs.

```
              Ingress (optional)
   /api, /health, /readyz │      │ everything else
            ┌─────────────▼┐    ┌▼──────────────┐
            │ API          │    │ Web dashboard │
            │ 2+ pods, HPA │    │ 2+ pods, HPA  │
            └──────┬───────┘    └───────────────┘
                   │
   ┌───────────────┼──────────────────────────┐
   │ Indexer: 1 pod ("single")                 │
   │   or 1 coordinator + N workers ("sharded")│
   └───────────────┬──────────────────────────┘
                   │
       external PostgreSQL 16  ·  external Redis 7
```

## Requirements

- Kubernetes 1.25 or later and Helm 3.12 or later.
- PostgreSQL 16 and Redis 7 reachable from the cluster.
- Images:
  - The API and indexer images are published to GHCR on every release
    (`ghcr.io/sorolens/sorolens-api`, `ghcr.io/sorolens/sorolens-indexer`).
    The chart defaults to the release matching its `appVersion`. They are
    signed; see [Verifying releases](../verifying-releases.md).
  - The dashboard has no published image, because Next.js inlines
    `NEXT_PUBLIC_API_URL` when it builds. Build one for your public URL and
    push it to a registry the cluster can pull from.
  - Migrations use a small image holding the SQL files (below).

## 1. Build the dashboard and migrations images

**Dashboard.** Build a Next.js server image of `apps/web` with
`NEXT_PUBLIC_API_URL` set to the URL users will open (for example
`https://sorolens.example.com`, the host the ingress below serves). The
container must serve on `$PORT` (3000) and answer `GET /`, the same contract
the [AWS guide](aws.md#1-build-and-push-the-images) describes. The chart runs
it as UID 65532 with a read-only root filesystem and mounts writable
`emptyDir` volumes at `web.writablePaths` (by default `/tmp` and
`/app/.next/cache`); set those to wherever your image keeps its `.next`
directory.

**Migrations.** The SQL files on a psql image, from the repository root:

```bash
REGISTRY=registry.example.com/sorolens
VERSION=0.1.0

cat > /tmp/Dockerfile.migrations <<'EOF'
FROM postgres:16-alpine
COPY apps/api/internal/db/migrations /migrations
EOF
docker build -f /tmp/Dockerfile.migrations -t $REGISTRY/sorolens-migrations:$VERSION .
docker push $REGISTRY/sorolens-migrations:$VERSION
```

The Job's default command mirrors the repository's compose `migrate`
service: on a fresh database it applies every `*.up.sql` in order with `psql`,
and it exits without changes when the schema already exists. Override
`migrations.command` to use another tool.

## 2. Store the connection strings

Keep credentials out of your values file with a Secret of your own:

```bash
kubectl create namespace sorolens
kubectl -n sorolens create secret generic sorolens-connections \
  --from-literal=DATABASE_URL='postgres://sorolens:...@pgbouncer:6432/sorolens?sslmode=require' \
  --from-literal=DIRECT_DATABASE_URL='postgres://sorolens:...@postgres:5432/sorolens?sslmode=require' \
  --from-literal=REDIS_URL='rediss://:...@redis:6379/0'
```

`DIRECT_DATABASE_URL` bypasses a connection pooler for migrations. Without a
pooler, set `database.secretKeys.directDatabaseUrl: DATABASE_URL` instead of
adding it. The key names are configurable under `database.secretKeys`.

For a quick evaluation you can instead put the URLs in `database.url`,
`database.directUrl` and `database.redisUrl`; the chart then creates the
Secret itself.

## 3. Write a values file

```yaml
# sorolens-values.yaml
config:
  stellarNetwork: testnet          # testnet, futurenet or mainnet
  # sorobanRpcUrls:                # required for mainnet: there is no public default
  #   mainnet: https://your-rpc-provider.example
  allowedOrigins:
    - https://sorolens.example.com

database:
  existingSecret: sorolens-connections

web:
  image:
    repository: registry.example.com/sorolens/sorolens-web
    tag: "0.1.0"

migrations:
  enabled: true
  image:
    repository: registry.example.com/sorolens/sorolens-migrations
    tag: "0.1.0"

ingress:
  enabled: true
  className: nginx
  host: sorolens.example.com
  tls:
    enabled: true
    secretName: sorolens-tls       # e.g. issued by cert-manager
```

Every value is checked against
[`values.schema.json`](../../deploy/helm/sorolens/values.schema.json):
a typo in a key, an unknown network, `latest` as an image tag or a missing
dashboard image fails `helm install` before anything is created. The full
list of values with their defaults is in
[`values.yaml`](../../deploy/helm/sorolens/values.yaml).

## 4. Install

```bash
helm install sorolens deploy/helm/sorolens -n sorolens -f sorolens-values.yaml --wait
helm test sorolens -n sorolens
```

With `migrations.enabled`, a Job applies the schema before the API starts;
`--wait` returns once every pod is ready. `helm test` passes when the API's
`/readyz` reports that PostgreSQL and Redis are reachable.

Without the migrations Job, apply the schema yourself first, for example with
`make migrate-up DB_URL=...` from a machine that can reach the database.

## Upgrading

```bash
helm upgrade sorolens deploy/helm/sorolens -n sorolens -f sorolens-values.yaml --wait
```

- The migrations Job runs again before the new pods roll out. It skips work
  when the schema is already in place; a failed migration fails the upgrade
  and leaves the previous version serving. `helm rollback sorolens` undoes a
  bad release (it does not undo schema changes).
- The API and dashboard roll one pod at a time behind their
  PodDisruptionBudgets.
- The single indexer uses the `Recreate` strategy: it keeps its shard state
  in memory, so the old pod stops before the new one starts. Expect a short
  gap in indexing, which it catches up on.
- Pods restart automatically when the chart-managed Secret changes. With
  `existingSecret`, restart them yourself after rotating credentials:
  `kubectl -n sorolens rollout restart deploy -l app.kubernetes.io/instance=sorolens`.

## Configuration

| Area | Values | Notes |
| --- | --- | --- |
| Network | `config.stellarNetwork`, `config.sorobanRpcUrls` | testnet and futurenet default to the public SDF endpoints. The indexer indexes every network listed in `sorobanRpcUrls` plus `stellarNetwork`. |
| Indexer scale | `indexer.mode`, `indexer.sharding.*` | `single` (default) runs one pod. `sharded` runs one coordinator and `workers.replicas` workers that share shards through PostgreSQL; see [indexer sharding](../indexer-sharding.md). Keep `shardCount` fixed once running. |
| API and dashboard scale | `api.replicaCount`, `api.autoscaling.*`, `web.*` | Autoscaling uses CPU and needs the metrics server. |
| Resources | `*.resources` | Requests are set for every container; tune limits to your traffic. |
| Probes | `*.livenessProbe`, `*.readinessProbe` | The API is live on `/health` and ready on `/readyz`, which checks PostgreSQL and Redis. The indexer is probed on its `/metrics` endpoint. |
| Security | `podSecurityContext`, `securityContext` | Non-root (UID 65532), read-only root filesystem, no privilege escalation, all capabilities dropped, `RuntimeDefault` seccomp. No pod mounts a service account token. |
| Extra settings | `*.extraEnv` | Any other environment variable, e.g. a Sentry DSN from a Secret. |

## Checking the chart

`deploy/helm/test.sh` lints the chart, renders every scenario in
`deploy/helm/ci/` and validates the manifests with kubeconform, then checks
that invalid values are refused. It needs `helm` and `kubeconform` and runs
in the IaC workflow on pushes to `main`.

## Uninstalling

```bash
helm uninstall sorolens -n sorolens
```

This removes everything the chart created. Your database, Redis and the
`existingSecret` are untouched.
