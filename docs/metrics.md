# Metrics

Sorolens exposes operational metrics in the [Prometheus text exposition
format](https://prometheus.io/docs/instrumenting/exposition_formats/) on the
indexer's `/metrics` endpoint. This page documents the metrics that are
emitted today, how to scrape them, and how to add more.

## Endpoint

The indexer serves `/metrics` on the address in `INDEXER_METRICS_ADDR`
(default `:9100`). Set `INDEXER_METRICS_ADDR=` (empty) to disable the
endpoint.

```bash
curl -s http://localhost:9100/metrics | grep sorolens_indexer
```

The endpoint is owned by the indexer because the indexer is the component that
talks to each network's Soroban RPC and owns the per-network ledger cursor.

### Scrape config

```yaml
scrape_configs:
  - job_name: sorolens-indexer
    static_configs:
      - targets: ["localhost:9100"]
```

## Indexer metrics

Every metric carries a `network` label. Its value is the configured network
(`testnet`, `mainnet`, or `futurenet`); when the indexer is wired with a single
unnamed RPC client, the value is `default`.

| Metric | Type | Labels | Description |
| --- | --- | --- | --- |
| `sorolens_indexer_lag_ledgers` | Gauge | `network` | Number of ledgers between the network head and the last ledger the indexer has committed for that network: `latest_ledger - last_indexed_ledger`. **This is the primary indexer health signal.** |
| `sorolens_indexer_head_ledger` | Gauge | `network` | Latest ledger sequence reported by the network's Soroban RPC. |
| `sorolens_indexer_last_indexed_ledger` | Gauge | `network` | Last ledger sequence committed by the indexer for the network (its indexer cursor). |

Notes:

- Metrics are recomputed once per poll pass, after that pass has committed its
  network cursors, so `lag_ledgers` reflects the ledger tip the pass reached.
- `lag_ledgers` is clamped at zero; a cursor that briefly reads ahead of the
  head never reports a negative gauge.
- Values are emitted for **every configured network**, not only the network
  being indexed at that moment, so a network that has stalled is visible even
  while the others are healthy.

## API metrics

The API serves its own Prometheus endpoint at `GET /metrics` on the API port
(alongside `/health`), with Go runtime and process metrics plus:

| Metric | Type | Labels | Description |
| --- | --- | --- | --- |
| `sorolens_api_cache_requests_total` | Counter | `namespace`, `result` | Response cache lookups. `namespace` is `contracts` or `watchdog`; `result` is `hit` or `miss`. |
| `sorolens_api_cache_purges_total` | Counter | `namespace` | Cache namespace purges triggered by a successful write (e.g. `POST /api/v1/contracts` purges `contracts`). |

Hit ratio per namespace:

```promql
sum by (namespace) (rate(sorolens_api_cache_requests_total{result="hit"}[5m]))
  /
sum by (namespace) (rate(sorolens_api_cache_requests_total[5m]))
```

Cached responses also carry an `X-Cache: HIT|MISS` header, which is handy
when checking a single request with `curl -i`.

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

## Adding a metric

1. Add the collector in `services/indexer/internal/metrics` and register it on
   the recorder's registry in `New`.
2. Update it from the poller (see `observeNetworkLag` for the per-network lag
   pass), keeping updates best-effort so metrics never fail an indexing pass.
3. Document it in the table above.
4. Keep the `sorolens_indexer_` namespace and a `network` label so dashboards
   and alerts stay consistent across metrics.
