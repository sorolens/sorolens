# Sorolens Python SDK

Official, typed Python client for the [Sorolens](https://github.com/sorolens/sorolens)
Soroban indexing API. It mirrors the REST surface described in
[`docs/openapi.yaml`](../../docs/openapi.yaml) and ships:

* a **generated** low-level client (`sorolens._generated`) produced from the OpenAPI
  spec with [`openapi-python-client`](https://github.com/openapi-generators/openapi-python-client);
* a **hand-written** high-level facade (`sorolens.Client` / `sorolens.AsyncClient`) with
  namespaced resources and a small exception hierarchy;
* **sync and async** transports, both built on [`httpx`](https://www.python-httpx.org/).

> **Publishing status:** the package metadata, build config, tests, docs and the
> tag-triggered release workflow all live in this repository. The first upload to PyPI
> still has to be performed by a maintainer (see [Releasing](#releasing)).

## Installation

```bash
pip install sorolens
```

Until the first release is published on PyPI, install from the repository:

```bash
pip install "sorolens @ git+https://github.com/sorolens/sorolens.git#subdirectory=packages/python-sdk"
```

## Quickstart

```python
from sorolens import Client

with Client(api_key="sk_live_...") as client:
    # Global index statistics
    stats = client.stats.global_stats()
    print(stats.tracked_contracts, stats.total_events)

    # Paginated contract listing
    page = client.contracts.list(limit=10, network="testnet")
    for contract in page.contracts:
        print(contract.id, contract.status)

    # Per-contract data
    contract_id = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
    print(client.stats.contract(contract_id, window="7d"))
    print(client.events.list(contract_id, limit=20))
    print(client.forecast.get(contract_id, horizon=30))
```

The base URL defaults to `https://api.sorolens.xyz`; override it for local or
self-hosted deployments:

```python
client = Client(base_url="http://localhost:8080", api_key="sk_dev_...")
```

### Async

```python
import asyncio
from sorolens import AsyncClient


async def main() -> None:
    async with AsyncClient(api_key="sk_live_...") as client:
        stats = await client.stats.global_stats()
        page = await client.contracts.list(limit=10)
        print(stats.tracked_contracts, len(page.contracts))


asyncio.run(main())
```

### Authentication

Pass a scoped API key as `api_key`; it is sent as `Authorization: Bearer <token>`.
Read endpoints also work unauthenticated, while API-key management and watchlist
endpoints need the matching scope, admin role or user identity header:

```python
client = Client(api_key="sk_admin_...")
client.api_keys.create("ci-reader", ["read:contracts", "read:watchdog"])

# Watchlist endpoints identify the caller with X-User-ID.
client.watchlist.add(user_id="user_123", contract_id=contract_id)
```

### Errors

Every non-2xx response raises a subclass of `sorolens.SorolensError`:

| Exception | Status | Meaning |
| --- | --- | --- |
| `AuthenticationError` | 401 | Missing or invalid credential/identity |
| `ForbiddenError` | 403 | Missing scope or role |
| `NotFoundError` | 404 | Resource does not exist |
| `ValidationError` | 422 | Invalid query or body |
| `RateLimitError` | 429 | Rate limit exceeded |
| `APIError` | any other | Base class for API errors |

```python
from sorolens import Client, NotFoundError

with Client(api_key="sk_live_...") as client:
    try:
        client.contracts.get("C...")
    except NotFoundError as exc:
        print(exc.status_code, exc.message)
```

## Available resources

| Resource | Methods |
| --- | --- |
| `client.health` | `health()`, `readyz()` |
| `client.stats` | `global_stats()`, `contract(contract_id, window=...)` |
| `client.contracts` | `list(...)`, `get(contract_id)`, `register(contract_id, network, label=...)` |
| `client.events` | `list(contract_id, ...)`, `stream(contract_id)` |
| `client.invocations` | `list(contract_id, ...)` |
| `client.storage` | `list(contract_id, ...)` |
| `client.forecast` | `get(contract_id, horizon=...)` |
| `client.snapshots` | `get(contract_id, ledger=...)` |
| `client.api_keys` | `list()`, `create()`, `revoke()`, `list_admin()`, `create_admin()`, `revoke_admin()` |
| `client.watchlist` | `list(user_id)`, `add(...)`, `remove(...)`, `status(...)` |
| `client.watchdog` | `stats()`, `alerts(...)`, `contracts(...)`, `contract()`, `health()`, `alerts_for()` |

`AsyncClient` exposes the same names as coroutines.

## Development

```bash
cd packages/python-sdk
pip install -e ".[dev]"
pytest
ruff check src tests
```

The low-level client is generated, not edited by hand. Regenerate it from the
OpenAPI spec with:

```bash
./scripts/generate_client.sh
```

## Releasing

A release is published by pushing a tag named `python-sdk-v<version>` (for example
`python-sdk-v0.1.0`). The [`python-sdk` workflow](../../.github/workflows/python-sdk.yml)
builds the sdist/wheel and uploads it to PyPI.

The workflow uses PyPI [trusted publishing](https://docs.pypi.org/trusted-publishers/),
so before the first release a maintainer must:

1. Reserve the `sorolens` project name on PyPI.
2. Add a trusted publisher for this repository with workflow `python-sdk.yml` and
   environment `pypi`.
3. Bump `version` in this package's `pyproject.toml` and push the matching tag.

Until those steps are done, the release job cannot publish — everything else in this
directory (build metadata, tests, docs, workflow) is ready for it.

## Documentation

Quickstart and API reference are built with MkDocs and published to
[Read the Docs](https://sorolens.readthedocs.io/) (`mkdocs.yml`, `docs/`, and the
repository-level `.readthedocs.yaml`).

## License

MIT — see [`LICENSE`](../../LICENSE).
