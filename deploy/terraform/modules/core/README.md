# Sorolens core (Terraform module)

Cloud-agnostic Sorolens configuration shared by the per-cloud modules. It
creates **no resources**: it validates the app settings and returns the
environment each service reads, so every cloud module configures the API,
indexer and dashboard the same way.

Secrets (`DATABASE_URL`, `DIRECT_DATABASE_URL`, `REDIS_URL`) are never part of
these environments; cloud modules inject them from their secret store.

## Inputs

| Name | Type | Default | Description |
|---|---|---|---|
| `name` | `string` | `"sorolens"` | Name prefix for every resource of this deployment. Lowercase letters, digits and hyphens; starts with a letter; at most 20 characters so derived names (load balancer, target groups) stay within AWS limits. |
| `stellar_network` | `string` | `"testnet"` | Default Stellar network for the API (STELLAR_NETWORK): testnet, mainnet or futurenet. |
| `soroban_rpc_urls` | `map(string)` | `{}` | Soroban RPC endpoint per network, e.g. { testnet = "https://..." }. testnet and futurenet fall back to the public SDF endpoints; mainnet has no default because it should use a commercial provider. The indexer indexes every network listed here plus stellar_network. |
| `watchdog_contract_id` | `string` | `null` | Deployed sorolens-watchdog contract ID (56-character C... strkey). When set, the indexer treats that contract's events as watchdog telemetry (WATCHDOG_ENABLED=true). |
| `log_level` | `string` | `"info"` | API log level (LOG_LEVEL): debug, info, warn or error. |
| `allowed_origins` | `list(string)` | `[]` | Browser origins allowed to call the API (ALLOWED_ORIGINS, read by the API once #381 lands). Leave empty when the dashboard is served from the same origin as the API. |
| `public_url` | `string` | **required** | Public origin the dashboard and API are served from, e.g. https://sorolens.example.com. Used for the dashboard's NEXT_PUBLIC_API_URL. |
| `indexer_poll_interval` | `string` | `"5m"` | Sleep between indexer passes in continuous mode (Go duration, e.g. 5m). |
| `api_port` | `number` | `8080` | Port the API container listens on (PORT). |
| `dashboard_port` | `number` | `3000` | Port the dashboard container listens on. |
| `indexer_metrics_port` | `number` | `9100` | Port of the indexer's Prometheus /metrics endpoint (INDEXER_METRICS_ADDR). Not exposed outside the private network. |

## Outputs

| Name | Description |
|---|---|
| `name` | Validated name prefix for resources. |
| `api_environment` | Non-secret environment variables for the API container. DATABASE_URL, DIRECT_DATABASE_URL and REDIS_URL are secrets and are injected by the cloud module. |
| `indexer_environment` | Non-secret environment variables for the indexer container. DATABASE_URL and REDIS_URL are injected by the cloud module as secrets. |
| `dashboard_environment` | Environment variables for the dashboard container. |
| `indexer_command` | Arguments for the indexer binary: continuous mode with the configured poll interval. |
| `api_port` | API container port. |
| `dashboard_port` | Dashboard container port. |
| `indexer_metrics_port` | Indexer metrics port. |

## Tests

```sh
terraform init -backend=false
terraform test
```
