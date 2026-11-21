# Sorolens

Real-time monitoring, alerting and on-chain health checks for Soroban smart
contracts on Stellar.

Soroban RPC nodes keep events and transaction data for a limited window (7 days
by default). Sorolens indexes what your contracts do into Postgres, so you keep
their history, can query it over a REST API, and get alerted when a contract you
depend on is unhealthy.

## What it does

- **Contract event indexing**: every event a contract emits, with decoded topics
  and values, queryable over REST.
- **Invocation tracing**: per-transaction CPU instructions, memory, ledger I/O
  and fees.
- **Storage tracking**: every temporary, persistent and instance storage entry,
  with TTL health.
- **Watchdog monitoring**: the on-chain `sorolens-watchdog` contract lets a
  contract owner register a contract, push health status and raise alerts.
- **Alerting**: TTL expiry, unresponsive contracts, and
  [custom rules](rule-language.md) such as
  `fee_per_invocation > 0.5 XLM for 5m on network testnet`.
- **Multi-network**: testnet, mainnet and futurenet side by side.

## Where to start

| I want to... | Read |
| --- | --- |
| Run Sorolens locally and track a first contract | [Getting started](getting-started.md) |
| Run Sorolens in production | [Deployment guide](deployment.md), [Kubernetes](self-hosting/kubernetes.md), [AWS](self-hosting/aws.md) |
| Call the API | [API overview](api/index.md) and the [API reference](api/reference.md) |
| Use a client library | [SDKs](sdks/index.md) |
| Use the terminal | [CLI](cli/index.md) |
| Operate a deployment | [Runbook](runbook.md), [Metrics](metrics.md) |
| Improve these docs | [Editing these docs](contributing/docs.md) |

## Versions

These docs are versioned. **latest** follows `main`, **stable** is the newest
release, and every release tag has its own copy. Switch versions from the
Read the Docs menu at the bottom of each page.

The source lives in [`docs/`](https://github.com/sorolens/sorolens/blob/main/docs) of the
[sorolens/sorolens](https://github.com/sorolens/sorolens) repository.
