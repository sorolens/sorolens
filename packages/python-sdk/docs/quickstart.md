# Quickstart

## Install

```bash
pip install sorolens
```

## Create a client

```python
from sorolens import Client

client = Client(api_key="sk_live_...")  # base_url defaults to https://api.sorolens.xyz
```

Use it as a context manager so the underlying `httpx` connection pool is closed for you:

```python
with Client(api_key="sk_live_...") as client:
    stats = client.stats.global_stats()
```

## Read data

```python
contract_id = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"

# Global index statistics.
stats = client.stats.global_stats()

# Contracts, filtered by network and paged with an opaque cursor.
page = client.contracts.list(limit=10, network="testnet")
while page.next_cursor:
    page = client.contracts.list(limit=10, cursor=page.next_cursor)

# Events, invocations, storage and forecasts for one contract.
client.events.list(contract_id, limit=20, type="contract")
client.invocations.list(contract_id, status="SUCCESS")
client.storage.list(contract_id, durability="persistent")
client.forecast.get(contract_id, horizon=30)
client.snapshots.get(contract_id, ledger=51234)
```

## Async

```python
import asyncio
from sorolens import AsyncClient


async def main() -> None:
    async with AsyncClient(api_key="sk_live_...") as client:
        stats = await client.stats.global_stats()
        print(stats.tracked_contracts)


asyncio.run(main())
```

## Handle errors

```python
from sorolens import Client, NotFoundError, RateLimitError

with Client(api_key="sk_live_...") as client:
    try:
        client.contracts.get(contract_id)
    except NotFoundError as exc:
        print("no such contract:", exc.message)
    except RateLimitError:
        ...
```

## Watchdog

```python
client.watchdog.stats(network="mainnet")
client.watchdog.alerts(severity="Critical", limit=50)
client.watchdog.contracts(limit=50)
client.watchdog.alerts_for(contract_id, severity="Warning")
```
