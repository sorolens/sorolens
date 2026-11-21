# Sorolens Python SDK

Official, typed Python client for the [Sorolens](https://github.com/sorolens/sorolens)
Soroban indexing API. It mirrors the REST surface described in
[`docs/openapi.yaml`](../../docs/openapi.yaml) and ships:

- a **generated** low-level client (`sorolens._generated`) produced from the OpenAPI
  spec with [`openapi-python-client`](https://github.com/openapi-generators/openapi-python-client);
- a **hand-written** high-level facade (`sorolens.Client` / `sorolens.AsyncClient`) with
  namespaced resources and a small exception hierarchy;
- **sync and async** transports, both built on [`httpx`](https://www.python-httpx.org/).

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

### Retries and cancellation

Both clients retry safe requests (`GET`/`HEAD`) on `429`, `500`, `502`, `503` and
`504` by default. `Retry-After` is honoured, otherwise the delay is exponential
backoff with jitter. In the async client the delay is an `await asyncio.sleep`,
so cancelling the task stops the retry loop immediately.

```python
from sorolens import Client, RetryPolicy

client = Client(api_key="sk_...", retry=RetryPolicy(max_retries=5, max_backoff=20))
client = Client(api_key="sk_...", retry=False)  # disable
```

### Pagination iterators

`contracts`, `events` and `invocations` expose `iter(...)`, which follows
`next_cursor` until the last page. The async version is an async iterator and
accepts the same arguments as `list(...)`.

For events and invocations, `client.iter_events(contract_id, ...)` and
`client.iter_invocations(contract_id, ...)` (async iterators on `AsyncClient`)
walk every page for you. They, and `events.iter` / `invocations.iter`:

- fetch pages lazily: the next page is requested only once the current one is used up;
- stop on the first empty `next_cursor`;
- clamp `limit` (the page size) to the server maximum of 200, and reject a limit below 1;
- raise a failed request's error after yielding the items already received;
- raise `PaginationError` if the API hands back a cursor the walk already requested,
  instead of looping.

```python
for event in client.iter_events(contract_id, limit=200):
    ...

async for invocation in async_client.iter_invocations(contract_id, status="FAILED"):
    ...

for contract in client.contracts.iter(limit=100, network="testnet"):
    ...
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

| Exception             | Status    | Meaning                                |
| --------------------- | --------- | -------------------------------------- |
| `AuthenticationError` | 401       | Missing or invalid credential/identity |
| `ForbiddenError`      | 403       | Missing scope or role                  |
| `NotFoundError`       | 404       | Resource does not exist                |
| `ValidationError`     | 422       | Invalid query or body                  |
| `RateLimitError`      | 429       | Rate limit exceeded                    |
| `APIError`            | any other | Base class for API errors              |

```python
from sorolens import Client, NotFoundError

with Client(api_key="sk_live_...") as client:
    try:
        client.contracts.get("C...")
    except NotFoundError as exc:
        print(exc.status_code, exc.message)
```

## Available resources

| Resource             | Methods                                                                              |
| -------------------- | ------------------------------------------------------------------------------------ |
| `client.health`        | `health()`, `readyz()`                                                               |
| `client.stats`         | `global_stats()`, `contract(contract_id, window=...)`, `compare(ids, window=...)`    |
| `client.contracts`     | `list(...)`, `iter(...)`, `get(...)`, `register(...)`, `upgrades()`, `graph()`, `health_score()`, `summary()` |
| `client.events`        | `list(contract_id, ...)`, `iter(contract_id, ...)`, `stream(contract_id)`            |
| `client.invocations`   | `list(contract_id, ...)`, `iter(contract_id, ...)`                                   |
| `client.storage`       | `list(contract_id, ...)`                                                             |
| `client.forecast`      | `get(contract_id, horizon=...)`                                                      |
| `client.snapshots`     | `get(contract_id, ledger=...)`                                                       |
| `client.api_keys`      | `list()`, `create()`, `revoke()`, `list_admin()`, `create_admin()`, `revoke_admin()` |
| `client.watchlist`     | `list(user_id)`, `add(...)`, `remove(...)`, `status(...)`                            |
| `client.watchdog`      | `stats()`, `alerts(...)`, `contracts(...)`, `contract()`, `health()`, `alerts_for()` |
| `client.alerts`        | `list(flat=..., severity=..., network=..., cursor=..., limit=...)`                   |
| `client.search`        | `contracts(q, limit=...)`                                                            |
| `client.labels`        | `list(query=...)`, `create(label, value, scope=...)`, `resolve(query)`               |
| `client.dlq`           | `list(cursor=..., limit=...)`, `requeue(event_id)`                                   |
| `client.reports`       | `contract(...)`, `history(...)`, `badge(...)`, `create_subscription(...)`, `subscriptions(...)`, `delete_subscription(...)`, `unsubscribe(token)` |
| `client.notifications` | `list()`, `create(contract_id, channel_type=..., ...)`, `delete(subscription_id)`    |

`AsyncClient` exposes the same names as coroutines.

## Examples

Five runnable examples live in [`examples/`](examples). Each one runs offline
out of the box (canned responses through `httpx.MockTransport`) and against a
live API when `SOROLENS_API_URL` is set:

| Example                          | Shows                                                                            |
| -------------------------------- | -------------------------------------------------------------------------------- |
| [`pagination.py`](examples/pagination.py) | Walking `client.contracts.list` by hand, then `iter_events`/`iter_invocations`. |
| [`errors.py`](examples/errors.py)         | `NotFoundError`, `ValidationError` and `RateLimitError` from real responses. |
| [`sse.py`](examples/sse.py)               | Streaming `GET /api/v1/stream/events` incrementally with `httpx`.          |
| [`webhooks.py`](examples/webhooks.py)     | Verifying `X-Sorolens-Signature` HMAC-SHA256 webhook deliveries.           |
| [`async_retry.py`](examples/async_retry.py) | `AsyncClient` retry after `429`, then `async for` pagination.            |

```bash
cd packages/python-sdk
python examples/pagination.py
SOROLENS_API_URL=https://api.sorolens.xyz python examples/errors.py
```

CI runs every example, so they are always executable.

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
