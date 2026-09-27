# Metrics

Sorolens exposes operational metrics in the [Prometheus text exposition
format](https://prometheus.io/docs/instrumenting/exposition_formats/) from both
the API and the indexer. This page documents the metric names, their labels, and
how to scrape them.

- [Endpoints](#endpoints)
- [Metric catalogue](#metric-catalogue)
  - [API](#api)
  - [Indexer](#indexer)
  - [Default Go collectors](#default-go-collectors)
- [Example queries](#example-queries)
- [Scrape configuration](#scrape-configuration)
- [Alerting](#alerting)
- [Grafana dashboard](#grafana-dashboard)
- [Adding a metric](#adding-a-metric)

---

## Endpoints

| Service | Endpoint | Auth | Default address |
|---|---|---|---|
| API | `GET /metrics` | none | `PORT` (8080) |
| API admin listener | `GET /metrics` | none | `METRICS_PORT` (only if set) |
| Indexer | `GET /metrics` | none | `INDEXER_METRICS_ADDR` (`:9100`) |

Both services return the Prometheus text exposition format
(`Content-Type: text/plain; version=0.0.4`).

The API always serves `GET /metrics` on its main port, so the acceptance
criterion holds with a plain:

```bash
curl -s localhost:8080/metrics
```

Set `METRICS_PORT` to additionally serve the endpoint on a dedicated admin
listener, which is the recommended production setup: the port can be bound to a
private interface and excluded from the public ingress.

```bash
# API on :8080, metrics also on :9090
METRICS_PORT=9090 go run ./cmd/api
```

The indexer serves `/metrics` on `INDEXER_METRICS_ADDR` (default `:9100`). Set
`INDEXER_METRICS_ADDR=` (empty) to disable the endpoint.

```bash
curl -s http://localhost:9100/metrics | grep sorolens_indexer
```

The endpoint is owned by the indexer because the indexer is the component that
talks to each network's Soroban RPC and owns the per-network ledger cursor.

The scrape endpoint is exempt from the API rate limiter (`middleware.RateLimit`)
so that a scrape can never be throttled and never consumes another caller's
budget.

---

## Metric catalogue

All Sorolens metrics are prefixed `sorolens_` so they are easy to select in
PromQL:

```
{__name__=~"sorolens_.*"}
```

### API

| Metric | Type | Labels | Description |
|---|---|---|---|
| `sorolens_http_requests_total` | Counter | `method`, `route`, `status` | Completed HTTP requests. |
| `sorolens_http_request_duration_seconds` | Histogram | `method`, `route`, `status` | Request latency in seconds. |
| `sorolens_http_requests_in_flight` | Gauge | – | Requests currently being served. |
| `sorolens_api_cache_requests_total` | Counter | `namespace`, `result` | Response cache lookups. `namespace` is `contracts`, `watchdog`, or `labels`; `result` is `hit` or `miss`. |
| `sorolens_api_cache_purges_total` | Counter | `namespace` | Cache namespace purges triggered by a successful write (e.g. `POST /api/v1/contracts` purges `contracts`). |

The `route` label is the **chi route pattern**, not the raw request path — for
example `/api/v1/contracts/{id}` rather than
`/api/v1/contracts/CDLZFC3S...`. This keeps label cardinality bounded no matter
how many distinct contract IDs are requested. A request that matches no route
(a 404) is recorded as `route="unmatched"`.

Histogram buckets are the Prometheus defaults (`prometheus.DefBuckets`), from
0.005s to 10s.

Hit ratio per namespace:

```promql
sum by (namespace) (rate(sorolens_api_cache_requests_total{result="hit"}[5m]))
  /
sum by (namespace) (rate(sorolens_api_cache_requests_total[5m]))
```

Cached responses also carry an `X-Cache: HIT|MISS` header, which is handy when
checking a single request with `curl -i`.

### Indexer

Every metric carries a `network` label. Its value is the configured network
(`testnet`, `mainnet`, or `futurenet`); when the indexer is wired with a single
unnamed RPC client, the value is `default`.

| Metric | Type | Labels | Description |
| --- | --- | --- | --- |
| `sorolens_indexer_lag_ledgers` | Gauge | `network` | Number of ledgers between the network head and the last ledger the indexer has committed for that network: `latest_ledger - last_indexed_ledger`. **This is the primary indexer health signal.** |
| `sorolens_indexer_head_ledger` | Gauge | `network` | Latest ledger sequence reported by the network's Soroban RPC. |
| `sorolens_indexer_last_indexed_ledger` | Gauge | `network` | Last ledger sequence committed by the indexer for the network (its indexer cursor). |
| `sorolens_indexer_events_processed_total` | Counter | `network` | Contract events persisted. |
| `sorolens_indexer_run_duration_seconds` | Histogram | `mode` | Duration of a full indexer pass (`once` or `continuous`). |

Notes:

- Metrics are sampled after a batch has committed, so they always describe
  durable progress rather than in-flight work.
- `lag_ledgers` is clamped at zero; a cursor that briefly reads ahead of the
  head never reports a negative gauge.
- `events_processed_total` only counts events after the batch insert succeeds; a
  failed insert does not increment it, and a zero increment never creates a
  series.
- `run_duration_seconds` buckets extend to 300s to cover the `INDEXER_MAX_DURATION`
  default of 270s.

### Default Go collectors

Both services also register the standard Go and process collectors:

| Metric family | Description |
|---|---|
| `go_*` | Goroutines, GC pauses, heap and stack usage, `go_info`. |
| `process_*` | CPU seconds, open file descriptors, resident memory, start time. |

The most useful of these for operators are `go_goroutines` (leak detection),
`go_gc_duration_seconds`, and `process_resident_memory_bytes`.

---

## Example queries

**Request rate per route**

```promql
sum by (route) (rate(sorolens_http_requests_total[5m]))
```

**p95 latency per route**

```promql
histogram_quantile(
  0.95,
  sum by (route, le) (rate(sorolens_http_request_duration_seconds_bucket[5m]))
)
```

**5xx error ratio**

```promql
sum(rate(sorolens_http_requests_total{status=~"5.."}[5m]))
  /
sum(rate(sorolens_http_requests_total[5m]))
```

**Indexer falling behind** (alert when lag exceeds 100 ledgers for 10 minutes)

```promql
max by (network) (sorolens_indexer_lag_ledgers) > 100
```

**Indexing throughput**

```promql
sum by (network) (rate(sorolens_indexer_events_processed_total[5m]))
```

**Indexer run duration p99**

```promql
histogram_quantile(
  0.99,
  sum by (mode, le) (rate(sorolens_indexer_run_duration_seconds_bucket[1h]))
)
```

---

## Scrape configuration

```yaml
scrape_configs:
  - job_name: sorolens-api
    metrics_path: /metrics
    static_configs:
      - targets: ["api.internal:9090"]   # METRICS_PORT

  - job_name: sorolens-indexer
    metrics_path: /metrics
    static_configs:
      - targets: ["indexer.internal:9100"]   # INDEXER_METRICS_ADDR
```

Because the indexer runs as a cron job (GitHub Actions) in the default
deployment, its metrics are only scrapeable while a process is running. Run it
with `--mode continuous` to keep the endpoint up; see
[`ARCHITECTURE.md` § 5.1](../ARCHITECTURE.md) for the migration path from cron
to a persistent worker.

---

## Alerting

A minimal alert fires when the indexer falls behind the chain and stays there:

```yaml
groups:
  - name: sorolens-indexer
    rules:
      - alert: SorolensIndexerLagging
        expr: sorolens_indexer_lag_ledgers > 1000
        for: 15m
        labels:
          severity: warning
        annotations:
          summary: "Indexer is >{{ $value }} ledgers behind on {{ $labels.network }}"
```

Pick the threshold from your network's ledger time. On Stellar a ledger closes
roughly every 5 seconds, so 1000 ledgers is about 83 minutes of lag.

---

## Grafana dashboard

A ready-to-import dashboard is checked in at
[`docs/dashboards/sorolens-overview.json`](dashboards/sorolens-overview.json).

Import it via **Dashboards → New → Import → Upload JSON file**, then pick the
Prometheus datasource when prompted. The dashboard covers request rate, latency
percentiles, error ratio, in-flight requests, indexer lag, indexing throughput,
indexer pass duration, goroutines, and RSS.

---

## Adding a metric

1. Add the collector in the relevant `internal/metrics` package and register it
   on the recorder's registry in `New` (indexer) or the package registry
   (API).
2. Update it from the owner of the data (the poller for indexer metrics, the
   middleware for API request metrics), keeping updates best-effort so metrics
   never fail a request or an indexing pass.
3. Document it in the table above.
4. Keep the `sorolens_` namespace and stable labels (`network` for indexer
   metrics, `method`/`route`/`status` for HTTP metrics) so dashboards and alerts
   stay consistent.
