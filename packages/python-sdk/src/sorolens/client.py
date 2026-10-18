"""Hand-written high-level facade over the generated Sorolens API client.

The low-level, spec-generated layer lives in :mod:`sorolens._generated` (produced
by ``scripts/generate_client.sh`` from ``docs/openapi.yaml``).  This module wraps
it into the ergonomic ``sorolens.Client`` / ``sorolens.AsyncClient`` surface that
application code is expected to use: same request shapes, typed return values,
sync and async (``httpx``) transports, and a small exception hierarchy instead of
raw status codes.
"""

from __future__ import annotations

from typing import Any

from . import exceptions
from ._generated.api.api_keys import (
    create_api_key,
    create_api_key_admin,
    list_api_keys,
    list_api_keys_admin,
    revoke_api_key,
    revoke_api_key_admin,
)
from ._generated.api.contracts import get_contract, list_contracts, register_contract
from ._generated.api.events import list_contract_events, stream_contract_events
from ._generated.api.forecast import get_contract_forecast
from ._generated.api.health import health as health_operation
from ._generated.api.health import readyz
from ._generated.api.invocations import list_contract_invocations
from ._generated.api.snapshots import get_contract_snapshot
from ._generated.api.stats import get_contract_stats, get_global_stats
from ._generated.api.storage import list_contract_storage
from ._generated.api.watchdog import (
    get_monitored_contract,
    get_watchdog_stats,
    list_contract_alerts,
    list_contract_health_checks,
    list_monitored_contracts,
    list_watchdog_alerts,
)
from ._generated.api.watchlist import (
    add_to_watchlist,
    get_watchlist_status,
    list_watchlist,
    remove_from_watchlist,
)
from ._generated.client import AuthenticatedClient
from ._generated.client import Client as _GeneratedClient
from ._generated.models import (
    AddToWatchlistBody,
    Contract,
    ContractSnapshot,
    ContractStats,
    CreateApiKeyAdminBody,
    CreateApiKeyBody,
    CreateApiKeyBodyScopesItem,
    GetContractForecastResponse200,
    GetGlobalStatsResponse200,
    GetWatchdogStatsNetwork,
    HealthResponse200,
    InWatchlist,
    ListApiKeysResponse200,
    ListContractAlertsNetwork,
    ListContractAlertsResponse200,
    ListContractAlertsSeverity,
    ListContractEventsNetwork,
    ListContractEventsResponse200,
    ListContractHealthChecksResponse200,
    ListContractInvocationsNetwork,
    ListContractInvocationsResponse200,
    ListContractInvocationsStatus,
    ListContractsNetwork,
    ListContractsResponse200,
    ListContractStorageDurability,
    ListContractStorageNetwork,
    ListContractStorageResponse200,
    ListContractStorageStatus,
    ListMonitoredContractsNetwork,
    ListMonitoredContractsResponse200,
    ListWatchdogAlertsNetwork,
    ListWatchdogAlertsResponse200,
    ListWatchdogAlertsSeverity,
    MonitoredContract,
    RegisterContractBody,
    RegisterContractBodyNetwork,
    StreamContractEventsResponse200,
    WatchdogStats,
    Watchlist,
)
from ._generated.types import UNSET, Response

DEFAULT_BASE_URL = "https://api.sorolens.xyz"

__all__ = ["Client", "AsyncClient", "DEFAULT_BASE_URL"]


def _build_client(
    *,
    api_key: str | None,
    base_url: str,
    timeout: Any = None,
    headers: dict[str, str] | None = None,
    **httpx_args: Any,
) -> _GeneratedClient:
    """Return a generated client, authenticated when an API key is supplied."""
    common: dict[str, Any] = {"base_url": base_url, **httpx_args}
    if timeout is not None:
        common["timeout"] = timeout
    if headers:
        common["headers"] = headers
    if api_key:
        return AuthenticatedClient(token=api_key, **common)
    return _GeneratedClient(**common)


def _unwrap(response: Response[Any]) -> Any:
    """Raise the mapped error (if any) and return the parsed payload."""
    exceptions.raise_for_response(response.status_code, response.parsed, response.content)
    return response.parsed


def _value(value: Any) -> Any:
    return UNSET if value is None else value


def _enum(enum_type: Any, value: Any) -> Any:
    return UNSET if value is None else enum_type(value)


class _SyncResource:
    def __init__(self, client: Client) -> None:
        self._client = client._client


class _AsyncResource:
    def __init__(self, client: AsyncClient) -> None:
        self._client = client._client


# --------------------------------------------------------------------------- #
# Synchronous resources
# --------------------------------------------------------------------------- #
class _Health(_SyncResource):
    def health(self) -> HealthResponse200:
        """Liveness probe. Returns the parsed body (``status: ok``)."""
        return _unwrap(health_operation.sync_detailed(client=self._client))

    def readyz(self) -> Any:
        """Readiness probe. Returns the parsed body for both 200 and 503."""
        return readyz.sync_detailed(client=self._client).parsed


class _Stats(_SyncResource):
    def global_stats(self) -> GetGlobalStatsResponse200:
        """Counts across every tracked contract."""
        return _unwrap(get_global_stats.sync_detailed(client=self._client))

    def contract(self, contract_id: str, *, window: str | None = None) -> ContractStats:
        """Per-contract counters; ``window`` accepts ``"24h"``, ``"7d"`` or ``"30d"``."""
        return _unwrap(
            get_contract_stats.sync_detailed(
                contract_id, client=self._client, window=_value(window)
            )
        )


class _Contracts(_SyncResource):
    def list(
        self,
        *,
        cursor: str | None = None,
        limit: int | None = None,
        network: str | None = None,
        status: str | None = None,
    ) -> ListContractsResponse200:
        return _unwrap(
            list_contracts.sync_detailed(
                client=self._client,
                cursor=_value(cursor),
                limit=_value(limit),
                network=_enum(ListContractsNetwork, network),
                status=_value(status),
            )
        )

    def get(self, contract_id: str) -> Contract:
        return _unwrap(get_contract.sync_detailed(contract_id, client=self._client))

    def register(self, contract_id: str, network: str, *, label: str | None = None) -> Contract:
        body = RegisterContractBody(
            id=contract_id,
            network=RegisterContractBodyNetwork(network),
            label=_value(label),
        )
        return _unwrap(register_contract.sync_detailed(client=self._client, body=body))


class _Events(_SyncResource):
    def list(
        self,
        contract_id: str,
        *,
        cursor: str | None = None,
        limit: int | None = None,
        network: str | None = None,
        type: str | None = None,
        from_ledger: int | None = None,
        to_ledger: int | None = None,
    ) -> ListContractEventsResponse200:
        return _unwrap(
            list_contract_events.sync_detailed(
                contract_id,
                client=self._client,
                cursor=_value(cursor),
                limit=_value(limit),
                network=_enum(ListContractEventsNetwork, network),
                type_=_value(type),
                from_=_value(from_ledger),
                to=_value(to_ledger),
            )
        )

    def stream(self, contract_id: str) -> StreamContractEventsResponse200:
        """Short-poll the 20 most recent events for a contract."""
        return _unwrap(stream_contract_events.sync_detailed(contract_id, client=self._client))


class _Invocations(_SyncResource):
    def list(
        self,
        contract_id: str,
        *,
        cursor: str | None = None,
        limit: int | None = None,
        network: str | None = None,
        status: str | None = None,
        fn: str | None = None,
        from_ledger: int | None = None,
        to_ledger: int | None = None,
    ) -> ListContractInvocationsResponse200:
        return _unwrap(
            list_contract_invocations.sync_detailed(
                contract_id,
                client=self._client,
                cursor=_value(cursor),
                limit=_value(limit),
                network=_enum(ListContractInvocationsNetwork, network),
                status=_enum(ListContractInvocationsStatus, status),
                fn=_value(fn),
                from_=_value(from_ledger),
                to=_value(to_ledger),
            )
        )


class _Storage(_SyncResource):
    def list(
        self,
        contract_id: str,
        *,
        cursor: str | None = None,
        limit: int | None = None,
        network: str | None = None,
        durability: str | None = None,
        status: str | None = None,
    ) -> ListContractStorageResponse200:
        return _unwrap(
            list_contract_storage.sync_detailed(
                contract_id,
                client=self._client,
                cursor=_value(cursor),
                limit=_value(limit),
                network=_enum(ListContractStorageNetwork, network),
                durability=_enum(ListContractStorageDurability, durability),
                status=_enum(ListContractStorageStatus, status),
            )
        )


class _Forecast(_SyncResource):
    def get(
        self, contract_id: str, *, horizon: int | None = None
    ) -> GetContractForecastResponse200:
        """Forecast fees, invocations and events for the next ``horizon`` days."""
        return _unwrap(
            get_contract_forecast.sync_detailed(
                contract_id, client=self._client, horizon=_value(horizon)
            )
        )


class _Snapshots(_SyncResource):
    def get(self, contract_id: str, *, ledger: int) -> ContractSnapshot:
        return _unwrap(
            get_contract_snapshot.sync_detailed(contract_id, client=self._client, ledger=ledger)
        )


class _ApiKeys(_SyncResource):
    def list(
        self, *, cursor: str | None = None, limit: int | None = None
    ) -> ListApiKeysResponse200:
        return _unwrap(
            list_api_keys.sync_detailed(
                client=self._client, cursor=_value(cursor), limit=_value(limit)
            )
        )

    def create(self, name: str, scopes: list[str]) -> Any:
        body = CreateApiKeyBody(
            name=name, scopes=[CreateApiKeyBodyScopesItem(scope) for scope in scopes]
        )
        return _unwrap(create_api_key.sync_detailed(client=self._client, body=body))

    def revoke(self, key_id: str) -> None:
        _unwrap(revoke_api_key.sync_detailed(key_id, client=self._client))

    def list_admin(
        self, *, cursor: str | None = None, limit: int | None = None
    ) -> ListApiKeysResponse200:
        return _unwrap(
            list_api_keys_admin.sync_detailed(
                client=self._client, cursor=_value(cursor), limit=_value(limit)
            )
        )

    def create_admin(self, name: str, scopes: list[str]) -> Any:
        body = CreateApiKeyAdminBody(name=name, scopes=list(scopes))
        return _unwrap(create_api_key_admin.sync_detailed(client=self._client, body=body))

    def revoke_admin(self, key_id: str) -> None:
        _unwrap(revoke_api_key_admin.sync_detailed(key_id, client=self._client))


class _Watchlist(_SyncResource):
    def list(self, user_id: str) -> Watchlist:
        return _unwrap(list_watchlist.sync_detailed(client=self._client, x_user_id=user_id))

    def add(self, user_id: str, contract_id: str) -> InWatchlist:
        body = AddToWatchlistBody(contract_id=contract_id)
        return _unwrap(
            add_to_watchlist.sync_detailed(client=self._client, body=body, x_user_id=user_id)
        )

    def remove(self, user_id: str, contract_id: str) -> InWatchlist:
        return _unwrap(
            remove_from_watchlist.sync_detailed(contract_id, client=self._client, x_user_id=user_id)
        )

    def status(self, user_id: str, contract_id: str) -> InWatchlist:
        return _unwrap(
            get_watchlist_status.sync_detailed(contract_id, client=self._client, x_user_id=user_id)
        )


class _Watchdog(_SyncResource):
    def stats(self, *, network: str | None = None) -> WatchdogStats:
        return _unwrap(
            get_watchdog_stats.sync_detailed(
                client=self._client, network=_enum(GetWatchdogStatsNetwork, network)
            )
        )

    def alerts(
        self,
        *,
        severity: str | None = None,
        network: str | None = None,
        limit: int | None = None,
    ) -> ListWatchdogAlertsResponse200:
        return _unwrap(
            list_watchdog_alerts.sync_detailed(
                client=self._client,
                severity=_enum(ListWatchdogAlertsSeverity, severity),
                network=_enum(ListWatchdogAlertsNetwork, network),
                limit=_value(limit),
            )
        )

    def contracts(
        self,
        *,
        cursor: str | None = None,
        limit: int | None = None,
        network: str | None = None,
    ) -> ListMonitoredContractsResponse200:
        return _unwrap(
            list_monitored_contracts.sync_detailed(
                client=self._client,
                cursor=_value(cursor),
                limit=_value(limit),
                network=_enum(ListMonitoredContractsNetwork, network),
            )
        )

    def contract(self, contract_id: str) -> MonitoredContract:
        return _unwrap(get_monitored_contract.sync_detailed(contract_id, client=self._client))

    def health(
        self, contract_id: str, *, limit: int | None = None
    ) -> ListContractHealthChecksResponse200:
        return _unwrap(
            list_contract_health_checks.sync_detailed(
                contract_id, client=self._client, limit=_value(limit)
            )
        )

    def alerts_for(
        self,
        contract_id: str,
        *,
        severity: str | None = None,
        network: str | None = None,
        limit: int | None = None,
    ) -> ListContractAlertsResponse200:
        return _unwrap(
            list_contract_alerts.sync_detailed(
                contract_id,
                client=self._client,
                severity=_enum(ListContractAlertsSeverity, severity),
                network=_enum(ListContractAlertsNetwork, network),
                limit=_value(limit),
            )
        )


class Client:
    """Synchronous client for the Sorolens API.

    Usage::

        from sorolens import Client

        with Client(api_key="sk_...") as client:
            stats = client.stats.global_stats()
            page = client.contracts.list(limit=10)
    """

    def __init__(
        self,
        *,
        api_key: str | None = None,
        base_url: str = DEFAULT_BASE_URL,
        timeout: Any = None,
        headers: dict[str, str] | None = None,
        **httpx_args: Any,
    ) -> None:
        self._client = _build_client(
            api_key=api_key, base_url=base_url, timeout=timeout, headers=headers, **httpx_args
        )
        self.health = _Health(self)
        self.stats = _Stats(self)
        self.contracts = _Contracts(self)
        self.events = _Events(self)
        self.invocations = _Invocations(self)
        self.storage = _Storage(self)
        self.forecast = _Forecast(self)
        self.snapshots = _Snapshots(self)
        self.api_keys = _ApiKeys(self)
        self.watchlist = _Watchlist(self)
        self.watchdog = _Watchdog(self)

    def close(self) -> None:
        self._client.get_httpx_client().close()

    def __enter__(self) -> Client:
        self._client.get_httpx_client().__enter__()
        return self

    def __exit__(self, *exc_info: Any) -> None:
        self._client.get_httpx_client().__exit__(*exc_info)


# --------------------------------------------------------------------------- #
# Asynchronous resources
# --------------------------------------------------------------------------- #
class _AsyncHealth(_AsyncResource):
    async def health(self) -> HealthResponse200:
        return _unwrap(await health_operation.asyncio_detailed(client=self._client))

    async def readyz(self) -> Any:
        return (await readyz.asyncio_detailed(client=self._client)).parsed


class _AsyncStats(_AsyncResource):
    async def global_stats(self) -> GetGlobalStatsResponse200:
        return _unwrap(await get_global_stats.asyncio_detailed(client=self._client))

    async def contract(self, contract_id: str, *, window: str | None = None) -> ContractStats:
        return _unwrap(
            await get_contract_stats.asyncio_detailed(
                contract_id, client=self._client, window=_value(window)
            )
        )


class _AsyncContracts(_AsyncResource):
    async def list(
        self,
        *,
        cursor: str | None = None,
        limit: int | None = None,
        network: str | None = None,
        status: str | None = None,
    ) -> ListContractsResponse200:
        return _unwrap(
            await list_contracts.asyncio_detailed(
                client=self._client,
                cursor=_value(cursor),
                limit=_value(limit),
                network=_enum(ListContractsNetwork, network),
                status=_value(status),
            )
        )

    async def get(self, contract_id: str) -> Contract:
        return _unwrap(await get_contract.asyncio_detailed(contract_id, client=self._client))

    async def register(
        self, contract_id: str, network: str, *, label: str | None = None
    ) -> Contract:
        body = RegisterContractBody(
            id=contract_id,
            network=RegisterContractBodyNetwork(network),
            label=_value(label),
        )
        return _unwrap(await register_contract.asyncio_detailed(client=self._client, body=body))


class _AsyncEvents(_AsyncResource):
    async def list(
        self,
        contract_id: str,
        *,
        cursor: str | None = None,
        limit: int | None = None,
        network: str | None = None,
        type: str | None = None,
        from_ledger: int | None = None,
        to_ledger: int | None = None,
    ) -> ListContractEventsResponse200:
        return _unwrap(
            await list_contract_events.asyncio_detailed(
                contract_id,
                client=self._client,
                cursor=_value(cursor),
                limit=_value(limit),
                network=_enum(ListContractEventsNetwork, network),
                type_=_value(type),
                from_=_value(from_ledger),
                to=_value(to_ledger),
            )
        )

    async def stream(self, contract_id: str) -> StreamContractEventsResponse200:
        return _unwrap(
            await stream_contract_events.asyncio_detailed(contract_id, client=self._client)
        )


class _AsyncInvocations(_AsyncResource):
    async def list(
        self,
        contract_id: str,
        *,
        cursor: str | None = None,
        limit: int | None = None,
        network: str | None = None,
        status: str | None = None,
        fn: str | None = None,
        from_ledger: int | None = None,
        to_ledger: int | None = None,
    ) -> ListContractInvocationsResponse200:
        return _unwrap(
            await list_contract_invocations.asyncio_detailed(
                contract_id,
                client=self._client,
                cursor=_value(cursor),
                limit=_value(limit),
                network=_enum(ListContractInvocationsNetwork, network),
                status=_enum(ListContractInvocationsStatus, status),
                fn=_value(fn),
                from_=_value(from_ledger),
                to=_value(to_ledger),
            )
        )


class _AsyncStorage(_AsyncResource):
    async def list(
        self,
        contract_id: str,
        *,
        cursor: str | None = None,
        limit: int | None = None,
        network: str | None = None,
        durability: str | None = None,
        status: str | None = None,
    ) -> ListContractStorageResponse200:
        return _unwrap(
            await list_contract_storage.asyncio_detailed(
                contract_id,
                client=self._client,
                cursor=_value(cursor),
                limit=_value(limit),
                network=_enum(ListContractStorageNetwork, network),
                durability=_enum(ListContractStorageDurability, durability),
                status=_enum(ListContractStorageStatus, status),
            )
        )


class _AsyncForecast(_AsyncResource):
    async def get(
        self, contract_id: str, *, horizon: int | None = None
    ) -> GetContractForecastResponse200:
        return _unwrap(
            await get_contract_forecast.asyncio_detailed(
                contract_id, client=self._client, horizon=_value(horizon)
            )
        )


class _AsyncSnapshots(_AsyncResource):
    async def get(self, contract_id: str, *, ledger: int) -> ContractSnapshot:
        return _unwrap(
            await get_contract_snapshot.asyncio_detailed(
                contract_id, client=self._client, ledger=ledger
            )
        )


class _AsyncApiKeys(_AsyncResource):
    async def list(
        self, *, cursor: str | None = None, limit: int | None = None
    ) -> ListApiKeysResponse200:
        return _unwrap(
            await list_api_keys.asyncio_detailed(
                client=self._client, cursor=_value(cursor), limit=_value(limit)
            )
        )

    async def create(self, name: str, scopes: list[str]) -> Any:
        body = CreateApiKeyBody(
            name=name, scopes=[CreateApiKeyBodyScopesItem(scope) for scope in scopes]
        )
        return _unwrap(await create_api_key.asyncio_detailed(client=self._client, body=body))

    async def revoke(self, key_id: str) -> None:
        _unwrap(await revoke_api_key.asyncio_detailed(key_id, client=self._client))

    async def list_admin(
        self, *, cursor: str | None = None, limit: int | None = None
    ) -> ListApiKeysResponse200:
        return _unwrap(
            await list_api_keys_admin.asyncio_detailed(
                client=self._client, cursor=_value(cursor), limit=_value(limit)
            )
        )

    async def create_admin(self, name: str, scopes: list[str]) -> Any:
        body = CreateApiKeyAdminBody(name=name, scopes=list(scopes))
        return _unwrap(await create_api_key_admin.asyncio_detailed(client=self._client, body=body))

    async def revoke_admin(self, key_id: str) -> None:
        _unwrap(await revoke_api_key_admin.asyncio_detailed(key_id, client=self._client))


class _AsyncWatchlist(_AsyncResource):
    async def list(self, user_id: str) -> Watchlist:
        return _unwrap(
            await list_watchlist.asyncio_detailed(client=self._client, x_user_id=user_id)
        )

    async def add(self, user_id: str, contract_id: str) -> InWatchlist:
        body = AddToWatchlistBody(contract_id=contract_id)
        return _unwrap(
            await add_to_watchlist.asyncio_detailed(
                client=self._client, body=body, x_user_id=user_id
            )
        )

    async def remove(self, user_id: str, contract_id: str) -> InWatchlist:
        return _unwrap(
            await remove_from_watchlist.asyncio_detailed(
                contract_id, client=self._client, x_user_id=user_id
            )
        )

    async def status(self, user_id: str, contract_id: str) -> InWatchlist:
        return _unwrap(
            await get_watchlist_status.asyncio_detailed(
                contract_id, client=self._client, x_user_id=user_id
            )
        )


class _AsyncWatchdog(_AsyncResource):
    async def stats(self, *, network: str | None = None) -> WatchdogStats:
        return _unwrap(
            await get_watchdog_stats.asyncio_detailed(
                client=self._client, network=_enum(GetWatchdogStatsNetwork, network)
            )
        )

    async def alerts(
        self,
        *,
        severity: str | None = None,
        network: str | None = None,
        limit: int | None = None,
    ) -> ListWatchdogAlertsResponse200:
        return _unwrap(
            await list_watchdog_alerts.asyncio_detailed(
                client=self._client,
                severity=_enum(ListWatchdogAlertsSeverity, severity),
                network=_enum(ListWatchdogAlertsNetwork, network),
                limit=_value(limit),
            )
        )

    async def contracts(
        self,
        *,
        cursor: str | None = None,
        limit: int | None = None,
        network: str | None = None,
    ) -> ListMonitoredContractsResponse200:
        return _unwrap(
            await list_monitored_contracts.asyncio_detailed(
                client=self._client,
                cursor=_value(cursor),
                limit=_value(limit),
                network=_enum(ListMonitoredContractsNetwork, network),
            )
        )

    async def contract(self, contract_id: str) -> MonitoredContract:
        return _unwrap(
            await get_monitored_contract.asyncio_detailed(contract_id, client=self._client)
        )

    async def health(
        self, contract_id: str, *, limit: int | None = None
    ) -> ListContractHealthChecksResponse200:
        return _unwrap(
            await list_contract_health_checks.asyncio_detailed(
                contract_id, client=self._client, limit=_value(limit)
            )
        )

    async def alerts_for(
        self,
        contract_id: str,
        *,
        severity: str | None = None,
        network: str | None = None,
        limit: int | None = None,
    ) -> ListContractAlertsResponse200:
        return _unwrap(
            await list_contract_alerts.asyncio_detailed(
                contract_id,
                client=self._client,
                severity=_enum(ListContractAlertsSeverity, severity),
                network=_enum(ListContractAlertsNetwork, network),
                limit=_value(limit),
            )
        )


class AsyncClient:
    """Asynchronous (``httpx.AsyncClient``) client for the Sorolens API.

    Usage::

        from sorolens import AsyncClient

        async with AsyncClient(api_key="sk_...") as client:
            page = await client.contracts.list(limit=10)
    """

    def __init__(
        self,
        *,
        api_key: str | None = None,
        base_url: str = DEFAULT_BASE_URL,
        timeout: Any = None,
        headers: dict[str, str] | None = None,
        **httpx_args: Any,
    ) -> None:
        self._client = _build_client(
            api_key=api_key, base_url=base_url, timeout=timeout, headers=headers, **httpx_args
        )
        self.health = _AsyncHealth(self)
        self.stats = _AsyncStats(self)
        self.contracts = _AsyncContracts(self)
        self.events = _AsyncEvents(self)
        self.invocations = _AsyncInvocations(self)
        self.storage = _AsyncStorage(self)
        self.forecast = _AsyncForecast(self)
        self.snapshots = _AsyncSnapshots(self)
        self.api_keys = _AsyncApiKeys(self)
        self.watchlist = _AsyncWatchlist(self)
        self.watchdog = _AsyncWatchdog(self)

    async def close(self) -> None:
        await self._client.get_async_httpx_client().aclose()

    async def __aenter__(self) -> AsyncClient:
        await self._client.get_async_httpx_client().__aenter__()
        return self

    async def __aexit__(self, *exc_info: Any) -> None:
        await self._client.get_async_httpx_client().__aexit__(*exc_info)
