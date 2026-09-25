# Metrics

Sorolens exposes Prometheus metrics from the API and from the indexer. This
document is the reference for the metric names, their labels, and how to scrape
them.

- [Endpoints](#endpoints)
- [Metric catalogue](#metric-catalogue)
  - [API](#api)
  - [Indexer](#indexer)
  - [Default Go collectors](#default-go-collectors)
- [Example queries](#example-queries)
- [Scrape configuration](#scrape-configuration)
- [Grafana dashboard](#grafana-dashboard)

---

## Endpoints

| Service | Endpoint | Auth | Default port |
|---|---|---|---|
| API | `GET /metrics` | none | `PORT` (8080) |
| Indexer | `GET /metrics` | none | `METRICS_PORT` (only if set) |

Both endpoints return the Prometheus text exposition format
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

# Indexer in continuous mode, metrics on :9090
METRICS_PORT=9090 go run ./cmd/sorolens index --interval 5m
```

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

The `route` label is the **chi route pattern**, not the raw request path — for
example `/api/v1/contracts/{id}` rather than
`/api/v1/contracts/CDLZFC3S...`. This keeps label cardinality bounded no matter
how many distinct contract IDs are requested. A request that matches no route
(a 404) is recorded as `route="unmatched"`.

Histogram buckets are the Prometheus defaults (`prometheus.DefBuckets`), from
0.005s to 10s.

### Indexer

| Metric | Type | Labels | Description |
|---|---|---|---|
| `sorolens_indexer_ledger_lag` | Gauge | `network` | Ledgers between the observed chain head and the last ledger processed. `0` means caught up. |
| `sorolens_indexer_events_processed_total` | Counter | `network` | Contract events persisted. |
| `sorolens_indexer_run_duration_seconds` | Histogram | `mode` | Duration of a full indexer pass (`once` or `continuous`). |

Notes:

- `ledger_lag` is sampled after each contract batch is committed, so it always
  describes durable progress rather than in-flight work. Negative values are
  clamped to `0`, so a reorg cannot produce a negative lag.
- `events_processed_total` only counts events after
  `BatchInsertWithCursor` succeeds; a failed batch does not increment it.
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
max by (network) (sorolens_indexer_ledger_lag) > 100
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
      - targets: ["indexer.internal:9090"]
```

Because the indexer runs as a cron job (GitHub Actions) in the default
deployment, its metrics are only scrapeable while a process is running. Run it
with `--mode continuous` to keep the endpoint up; see
[`ARCHITECTURE.md` § 5.1](../ARCHITECTURE.md) for the migration path from cron
to a persistent worker.

---

## Grafana dashboard

A ready-to-import dashboard is checked in at
[`docs/dashboards/sorolens-overview.json`](dashboards/sorolens-overview.json).

Import it via **Dashboards → New → Import → Upload JSON file**, then pick the
Prometheus datasource when prompted. The dashboard covers request rate, latency
percentiles, error ratio, in-flight requests, indexer lag, indexing throughput,
and indexer pass duration.
