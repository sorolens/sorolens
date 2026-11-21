# Local observability stack

The repository ships a Docker Compose `observability` profile that starts
Prometheus and Grafana with the committed dashboard and alert rules already
provisioned. Nothing has to be imported by hand.

```bash
docker compose --profile observability up -d
./scripts/observability-smoke.sh
```

| Service | URL | Credentials |
| --- | --- | --- |
| Prometheus | <http://localhost:9090> | none (local only) |
| Grafana | <http://localhost:3001> | user `admin`, generated password |

Grafana is mapped to `3001` on the host because the dashboard's Next.js dev
server already uses `3000`. Both ports are bound to `127.0.0.1`, so the stack
is not reachable from other machines.

## Metrics sources

Prometheus scrapes four targets (`observability/prometheus/prometheus.yml`):

| Job | Target | What it provides |
| --- | --- | --- |
| `prometheus` | `localhost:9090` | Prometheus's own metrics |
| `grafana` | `grafana:3000` | Grafana's own metrics (unauthenticated `/metrics`) |
| `sorolens-api` | `host.docker.internal:8080` | Go runtime/process metrics plus `sorolens_api_cache_*` counters |
| `sorolens-indexer` | `host.docker.internal:9100` | `sorolens_indexer_*` lag, head and cursor gauges |

The API and indexer run on the host, not in Compose. Start them to populate the
dashboard:

```bash
make api                                  # API on :8080, /metrics included
cd services/indexer && go run .           # indexer, INDEXER_METRICS_ADDR=:9100
```

The full metric reference is [metrics.md](./metrics.md).

## What is provisioned

- **Datasource** `Prometheus` (uid `sorolens-prometheus`), the default for all
  panels: `observability/grafana/provisioning/datasources/prometheus.yml`.
- **Dashboard** `Sorolens Overview` in the `Sorolens` folder, loaded from
  `observability/grafana/dashboards/sorolens-overview.json` by the file
  provider in `observability/grafana/provisioning/dashboards/dashboards.yml`.
  It charts indexer lag per network, head vs indexed ledger, the API response
  cache hit ratio / lookup rate / purges, API goroutines and heap, and scrape
  target health.
- **Alert rules** in
  `observability/prometheus/rules/alerts.yml`, evaluated by Prometheus and
  visible under <http://localhost:9090/rules> and
  <http://localhost:9090/alerts>:
  - `SorolensIndexerLagging` — warning after 15m above 1000 ledgers.
  - `SorolensIndexerSeverelyLagging` — critical after 10m above 20000 ledgers.
  - `SorolensAPICacheHitRatioLow` — warning when the per-namespace hit ratio
    stays below 40% for 30m while traffic is flowing.

Alerts show up in Grafana's Alerting view only if Grafana is pointed at
Prometheus as a data source with alerting enabled; these rules are evaluated by
Prometheus itself, which keeps the configuration in the repository and works
without Grafana.

## Admin password

No default credential is used. On first start the one-shot `grafana-secrets`
service writes a random 32-byte password into the `sorolens_grafana_secrets`
volume, and Grafana reads it through `GF_SECURITY_ADMIN_PASSWORD__FILE`.

Read it after the stack is up:

```bash
docker compose exec grafana cat /etc/grafana-secrets/admin-password
```

To pick the password yourself, set it before the first start:

```bash
GRAFANA_ADMIN_PASSWORD='choose-a-strong-one' docker compose --profile observability up -d
```

The file is only written when it does not already exist, so a restart keeps the
existing password. To rotate it, remove the secret volume and start again:

```bash
docker compose --profile observability down
docker volume rm sorolens_grafana_secrets
```

## Smoke check

`scripts/observability-smoke.sh` (also `make observability-smoke`) verifies:

1. Prometheus answers `/-/ready`.
2. Both rule groups and all three alert rules are loaded and none is in an
   error state.
3. Every active scrape target reports `up` (it lists the down targets and
   exits non-zero otherwise).
4. Grafana is healthy, the Prometheus datasource is provisioned, and the
   `Sorolens Overview` dashboard is visible to the admin user.

Override the endpoints with `PROM_URL`, `GRAFANA_URL`, `GRAFANA_ADMIN_USER` and
`GRAFANA_ADMIN_PASSWORD` when running against non-default ports.

`make observability-check` validates the compose file, the Prometheus
configuration and rule files (`promtool`), and the dashboard JSON without
starting anything.

## Data and retention

Prometheus keeps 15 days of samples in the `sorolens_prometheus_data` volume;
Grafana state lives in `sorolens_grafana_data`. `docker compose --profile
observability down` stops the stack and keeps both. `down -v` for the whole
project deletes the Postgres, Redis and MinIO volumes too, so prefer removing
the observability volumes individually.
