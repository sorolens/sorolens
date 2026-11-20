"""Hand-written high-level facade over the generated Sorolens API client.

The low-level, spec-generated layer lives in :mod:`sorolens._generated` (produced
by ``scripts/generate_client.sh`` from ``docs/openapi.yaml``).  This module wraps
it into the ergonomic ``sorolens.Client`` / ``sorolens.AsyncClient`` surface that
application code is expected to use: same request shapes, typed return values,
sync and async (``httpx``) transports, and a small exception hierarchy instead of
raw status codes.
"""

from __future__ import annotations

import json
from collections.abc import AsyncIterator, Iterator, Sequence
from typing import Any

from . import exceptions
from ._generated.api.alerts import list_alerts
from ._generated.api.api_keys import (
    create_api_key,
    create_api_key_admin,
    list_api_keys,
    list_api_keys_admin,
    revoke_api_key,
    revoke_api_key_admin,
)
from ._generated.api.contracts import (
    get_api_v1_search,
    get_contract,
    get_contract_graph,
    list_contract_upgrades,
    list_contracts,
    register_contract,
)
from ._generated.api.default import (
    get_api_v1_labels,
    get_api_v1_resolve,
    post_api_v1_labels,
)
from ._generated.api.dlq import list_failed_events, requeue_failed_event
from ._generated.api.events import list_contract_events, stream_contract_events
from ._generated.api.forecast import get_contract_forecast
from ._generated.api.health import health as health_operation
from ._generated.api.health import readyz
from ._generated.api.invocations import list_contract_invocations
from ._generated.api.notifications import (
    create_alert_subscription,
    delete_alert_subscription,
    list_alert_subscriptions,
)
from ._generated.api.reports import (
    create_report_subscription,
    delete_report_subscription,
    get_contract_report,
    get_contract_report_history,
    get_contract_sla_badge,
    list_report_subscriptions,
    unsubscribe_report,
)
from ._generated.api.snapshots import get_contract_snapshot
from ._generated.api.stats import (
    compare_contracts,
    get_contract_health_score,
    get_contract_stats,
    get_contract_summary,
    get_global_stats,
)
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
    AlertSubscription,
    CompareContractsWindow,
    CompareResponse,
    Contract,
    ContractGraph,
    ContractReport,
    ContractSnapshot,
    ContractStats,
    ContractSummary,
    CreateAlertSubscription,
    CreateAlertSubscriptionChannelType,
    CreateAlertSubscriptionSeverityFilter,
    CreateApiKeyAdminBody,
    CreateApiKeyBody,
    CreateApiKeyBodyScopesItem,
    CreateReportSubscriptionBody,
    CreateReportSubscriptionBodyFrequency,
    GetApiV1SearchResponse200,
    GetContractForecastResponse200,
    GetContractReportFormat,
    GetContractReportHistoryResponse200,
    GetGlobalStatsResponse200,
    GetWatchdogStatsNetwork,
    HealthResponse200,
    HealthScore,
    InWatchlist,
    ListAlertsNetwork,
    ListAlertsResponse200,
    ListAlertsSeverity,
    ListAlertSubscriptionsResponse200,
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
    ListContractUpgradesResponse200,
    ListFailedEventsResponse200,
    ListMonitoredContractsNetwork,
    ListMonitoredContractsResponse200,
    ListReportSubscriptionsResponse200,
    ListWatchdogAlertsNetwork,
    ListWatchdogAlertsResponse200,
    ListWatchdogAlertsSeverity,
    MonitoredContract,
    PostApiV1LabelsBody,
    PostApiV1LabelsBodyScope,
    RegisterContractBody,
    RegisterContractBodyNetwork,
    ReportSubscription,
    RequeueFailedEventResponse200,
    StreamContractEventsResponse200,
    WatchdogStats,
    Watchlist,
)
from ._generated.types import UNSET, Response
from .retry import DEFAULT_RETRY_POLICY, RetryPolicy, RetryTransport

DEFAULT_BASE_URL = "https://api.sorolens.xyz"

__all__ = ["Client", "AsyncClient", "DEFAULT_BASE_URL"]


def _build_client(
    *,
    api_key: str | None,
    base_url: str,
    timeout: Any = None,
    headers: dict[str, str] | None = None,
    retry: RetryPolicy | bool | None = True,
    **httpx_args: Any,
) -> _GeneratedClient:
    """Return a generated client, authenticated when an API key is supplied.

    ``retry`` is on by default: safe requests (GET/HEAD) are retried on 429 and
    common 5xx responses, honouring ``Retry-After``. Pass ``False`` to disable
    it or a :class:`~sorolens.RetryPolicy` to tune it. A transport supplied in
    ``httpx_args`` is wrapped rather than replaced.
    """
    common: dict[str, Any] = {"base_url": base_url, **httpx_args}
    if retry is not False and retry is not None:
        policy = DEFAULT_RETRY_POLICY if retry is True else retry
        if not isinstance(policy, RetryPolicy):
            raise TypeError("retry must be a RetryPolicy, True, False or None")
        # ``httpx_args`` is the generated client's alias for the kwargs dict it
        # hands to httpx.Client/httpx.AsyncClient; keep any transport there.
        nested = common.get("httpx_args")
        if isinstance(nested, dict):
            nested = dict(nested)
            nested["transport"] = RetryTransport(nested.get("transport"), policy)
            common["httpx_args"] = nested
        else:
            common["transport"] = RetryTransport(common.get("transport"), policy)
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


def _payload(response: Response[Any]) -> Any:
    """Raise on error, then decode the JSON body of an untyped endpoint.

    A handful of operations (the label endpoints) declare no response schema,
    so the generated parser discards the body. Decode it here instead of
    returning ``None``.
    """
    exceptions.raise_for_response(response.status_code, response.parsed, response.content)
    if response.content:
        try:
            return json.loads(response.content)
        except ValueError:
            return response.content.decode()
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

    def compare(
        self, ids: str | Sequence[str], *, window: str | None = None
    ) -> CompareResponse:
        """Compare one to four contracts side by side; ``window`` is 24h/7d/30d."""
        joined = ids if isinstance(ids, str) else ",".join(ids)
        return _unwrap(
            compare_contracts.sync_detailed(
                client=self._client,
                ids=joined,
                window=_enum(CompareContractsWindow, window),
            )
        )

    def contract(self, contract_id: str, *, window: str | None = None) -> ContractStats:
        """Per-contract counters; ``window`` accepts ``"24h"``, ``"7d"`` or ``"30d"``."""
        return _unwrap(
            get_contract_stats.sync_detailed(
                contract_id, client=self._client, window=_value(window)
            )
        )


class _Contracts(_SyncResource):
    def iter(
        self,
        *,
        limit: int | None = None,
        network: str | None = None,
        status: str | None = None,
    ) -> Iterator[Contract]:
        """Yield every contract across all cursor pages."""
        cursor: str | None = None
        while True:
            page = self.list(cursor=cursor, limit=limit, network=network, status=status)
            yield from page.contracts
            if not page.next_cursor:
                return
            cursor = page.next_cursor

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

    def upgrades(
        self, contract_id: str, *, limit: int | None = None
    ) -> ListContractUpgradesResponse200:
        """List a contract's Wasm code upgrades, newest first."""
        return _unwrap(
            list_contract_upgrades.sync_detailed(
                contract_id, client=self._client, limit=_value(limit) or 50
            )
        )

    def graph(self, contract_id: str) -> ContractGraph:
        """Cross-contract call graph derived from the contract's recent invocations."""
        return _unwrap(get_contract_graph.sync_detailed(contract_id, client=self._client))

    def health_score(
        self, contract_id: str, *, history_limit: int | None = None
    ) -> HealthScore:
        """Composite 0-100 health score, optionally with hourly history snapshots."""
        return _unwrap(
            get_contract_health_score.sync_detailed(
                contract_id, client=self._client, history_limit=_value(history_limit)
            )
        )

    def summary(self, contract_id: str) -> ContractSummary:
        """Dashboard summary: metadata, stats and the latest event/invocation."""
        return _unwrap(get_contract_summary.sync_detailed(contract_id, client=self._client))


class _Events(_SyncResource):
    def iter(
        self,
        contract_id: str,
        *,
        limit: int | None = None,
        network: str | None = None,
        type: str | None = None,
        from_ledger: int | None = None,
        to_ledger: int | None = None,
    ) -> Iterator[Any]:
        """Yield every event across all cursor pages."""
        cursor: str | None = None
        while True:
            page = self.list(
                contract_id,
                cursor=cursor,
                limit=limit,
                network=network,
                type=type,
                from_ledger=from_ledger,
                to_ledger=to_ledger,
            )
            yield from page.events
            if not page.next_cursor:
                return
            cursor = page.next_cursor

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
    def iter(
        self,
        contract_id: str,
        *,
        limit: int | None = None,
        network: str | None = None,
        status: str | None = None,
        fn: str | None = None,
        from_ledger: int | None = None,
        to_ledger: int | None = None,
    ) -> Iterator[Any]:
        """Yield every invocation across all cursor pages."""
        cursor: str | None = None
        while True:
            page = self.list(
                contract_id,
                cursor=cursor,
                limit=limit,
                network=network,
                status=status,
                fn=fn,
                from_ledger=from_ledger,
                to_ledger=to_ledger,
            )
            yield from page.invocations
            if not page.next_cursor:
                return
            cursor = page.next_cursor

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


class _Alerts(_SyncResource):
    def list(
        self,
        *,
        flat: bool | None = None,
        contract_id: str | None = None,
        severity: str | None = None,
        network: str | None = None,
        cursor: str | None = None,
        limit: int | None = None,
    ) -> ListAlertsResponse200:
        """List alert groups; pass ``flat=True`` for the raw per-event feed."""
        return _unwrap(
            list_alerts.sync_detailed(
                client=self._client,
                flat=_value(flat),
                contract_id=_value(contract_id),
                severity=_enum(ListAlertsSeverity, severity),
                network=_enum(ListAlertsNetwork, network),
                cursor=_value(cursor),
                limit=_value(limit),
            )
        )


class _Search(_SyncResource):
    def contracts(self, q: str, *, limit: int = 10) -> GetApiV1SearchResponse200:
        """Search tracked contracts by ID or label."""
        return _unwrap(get_api_v1_search.sync_detailed(client=self._client, q=q, limit=limit))


class _Labels(_SyncResource):
    def list(self, *, query: str | None = None) -> Any:
        """Search visible account labels, optionally narrowed by ``query``.

        Returns the decoded JSON body (the operation declares no response
        schema in ``docs/openapi.yaml``).
        """
        return _payload(
            get_api_v1_labels.sync_detailed(client=self._client, query=_value(query))
        )

    def create(self, label: str, value: str, *, scope: str = "public") -> Any:
        """Create or update a label; ``scope`` is ``"public"`` or ``"workspace"``."""
        body = PostApiV1LabelsBody(
            label=label,
            value=value,
            scope=PostApiV1LabelsBodyScope(scope),
        )
        return _payload(post_api_v1_labels.sync_detailed(client=self._client, body=body))

    def resolve(self, query: str) -> Any:
        """Resolve a visible label to its Stellar identifier."""
        return _payload(
            get_api_v1_resolve.sync_detailed(client=self._client, query=query)
        )


class _Dlq(_SyncResource):
    def list(
        self, *, cursor: str | None = None, limit: int | None = None
    ) -> ListFailedEventsResponse200:
        """List dead-lettered indexer events, newest first."""
        return _unwrap(
            list_failed_events.sync_detailed(
                client=self._client, cursor=_value(cursor), limit=_value(limit)
            )
        )

    def requeue(self, event_id: int) -> RequeueFailedEventResponse200:
        """Requeue a dead-lettered event by its DLQ row id."""
        return _unwrap(requeue_failed_event.sync_detailed(event_id, client=self._client))


class _Reports(_SyncResource):
    def contract(
        self,
        contract_id: str,
        *,
        month: str | None = None,
        format_: str | None = None,
    ) -> ContractReport:
        """Monthly SLA/uptime report; ``format_`` is ``json``, ``csv`` or ``pdf``."""
        return _unwrap(
            get_contract_report.sync_detailed(
                contract_id,
                client=self._client,
                month=_value(month),
                format_=_enum(GetContractReportFormat, format_),
            )
        )

    def history(
        self, contract_id: str, *, months: int | None = None
    ) -> GetContractReportHistoryResponse200:
        """SLA history across the trailing ``months`` months."""
        return _unwrap(
            get_contract_report_history.sync_detailed(
                contract_id, client=self._client, months=_value(months)
            )
        )

    def badge(self, contract_id: str, *, month: str | None = None) -> str:
        """Shields.io-style SVG badge for a month's uptime."""
        response = get_contract_sla_badge.sync_detailed(
            contract_id, client=self._client, month=_value(month)
        )
        exceptions.raise_for_response(response.status_code, response.parsed, response.content)
        return response.content.decode("utf-8")

    def create_subscription(
        self, email: str, frequency: str, *, day_of_week: int | None = None
    ) -> ReportSubscription:
        """Subscribe an email to daily or weekly digest reports."""
        body = CreateReportSubscriptionBody(
            email=email,
            frequency=CreateReportSubscriptionBodyFrequency(frequency),
            day_of_week=_value(day_of_week),
        )
        return _unwrap(
            create_report_subscription.sync_detailed(client=self._client, body=body)
        )

    def subscriptions(self, email: str) -> ListReportSubscriptionsResponse200:
        """Active digest subscriptions for an email address."""
        return _unwrap(
            list_report_subscriptions.sync_detailed(client=self._client, email=email)
        )

    def delete_subscription(self, subscription_id: str) -> None:
        """Delete a digest subscription."""
        _unwrap(
            delete_report_subscription.sync_detailed(subscription_id, client=self._client)
        )

    def unsubscribe(self, token: str) -> str:
        """One-click unsubscribe from digest emails."""
        return _unwrap(unsubscribe_report.sync_detailed(client=self._client, token=token))


class _Notifications(_SyncResource):
    def list(self) -> ListAlertSubscriptionsResponse200:
        """List alert notification subscriptions, newest first."""
        return _unwrap(list_alert_subscriptions.sync_detailed(client=self._client))

    def create(
        self,
        contract_id: str,
        *,
        channel_type: str = "webhook",
        webhook_url: str | None = None,
        routing_key: str | None = None,
        severity_filter: str = "Critical",
    ) -> AlertSubscription:
        """Subscribe a channel (webhook/slack/discord/pagerduty) to alerts."""
        body = CreateAlertSubscription(
            contract_id=contract_id,
            channel_type=CreateAlertSubscriptionChannelType(channel_type),
            webhook_url=_value(webhook_url),
            routing_key=_value(routing_key),
            severity_filter=CreateAlertSubscriptionSeverityFilter(severity_filter),
        )
        return _unwrap(
            create_alert_subscription.sync_detailed(client=self._client, body=body)
        )

    def delete(self, subscription_id: str) -> None:
        """Delete an alert subscription."""
        _unwrap(
            delete_alert_subscription.sync_detailed(subscription_id, client=self._client)
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
        retry: RetryPolicy | bool | None = True,
        **httpx_args: Any,
    ) -> None:
        self._client = _build_client(
            api_key=api_key,
            base_url=base_url,
            timeout=timeout,
            headers=headers,
            retry=retry,
            **httpx_args,
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
        self.alerts = _Alerts(self)
        self.search = _Search(self)
        self.labels = _Labels(self)
        self.dlq = _Dlq(self)
        self.reports = _Reports(self)
        self.notifications = _Notifications(self)

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

    async def compare(
        self, ids: str | Sequence[str], *, window: str | None = None
    ) -> CompareResponse:
        """Compare one to four contracts side by side; ``window`` is 24h/7d/30d."""
        joined = ids if isinstance(ids, str) else ",".join(ids)
        return _unwrap(
            await compare_contracts.asyncio_detailed(
                client=self._client,
                ids=joined,
                window=_enum(CompareContractsWindow, window),
            )
        )

    async def contract(self, contract_id: str, *, window: str | None = None) -> ContractStats:
        return _unwrap(
            await get_contract_stats.asyncio_detailed(
                contract_id, client=self._client, window=_value(window)
            )
        )


class _AsyncContracts(_AsyncResource):
    async def iter(
        self,
        *,
        limit: int | None = None,
        network: str | None = None,
        status: str | None = None,
    ) -> AsyncIterator[Contract]:
        """Yield every contract across all cursor pages."""
        cursor: str | None = None
        while True:
            page = await self.list(cursor=cursor, limit=limit, network=network, status=status)
            for contract in page.contracts:
                yield contract
            if not page.next_cursor:
                return
            cursor = page.next_cursor

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

    async def upgrades(
        self, contract_id: str, *, limit: int | None = None
    ) -> ListContractUpgradesResponse200:
        """List a contract's Wasm code upgrades, newest first."""
        return _unwrap(
            await list_contract_upgrades.asyncio_detailed(
                contract_id, client=self._client, limit=_value(limit) or 50
            )
        )

    async def graph(self, contract_id: str) -> ContractGraph:
        """Cross-contract call graph derived from the contract's recent invocations."""
        return _unwrap(
            await get_contract_graph.asyncio_detailed(contract_id, client=self._client)
        )

    async def health_score(
        self, contract_id: str, *, history_limit: int | None = None
    ) -> HealthScore:
        """Composite 0-100 health score, optionally with hourly history snapshots."""
        return _unwrap(
            await get_contract_health_score.asyncio_detailed(
                contract_id, client=self._client, history_limit=_value(history_limit)
            )
        )

    async def summary(self, contract_id: str) -> ContractSummary:
        """Dashboard summary: metadata, stats and the latest event/invocation."""
        return _unwrap(
            await get_contract_summary.asyncio_detailed(contract_id, client=self._client)
        )


class _AsyncEvents(_AsyncResource):
    async def iter(
        self,
        contract_id: str,
        *,
        limit: int | None = None,
        network: str | None = None,
        type: str | None = None,
        from_ledger: int | None = None,
        to_ledger: int | None = None,
    ) -> AsyncIterator[Any]:
        """Yield every event across all cursor pages."""
        cursor: str | None = None
        while True:
            page = await self.list(
                contract_id,
                cursor=cursor,
                limit=limit,
                network=network,
                type=type,
                from_ledger=from_ledger,
                to_ledger=to_ledger,
            )
            for event in page.events:
                yield event
            if not page.next_cursor:
                return
            cursor = page.next_cursor

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
    async def iter(
        self,
        contract_id: str,
        *,
        limit: int | None = None,
        network: str | None = None,
        status: str | None = None,
        fn: str | None = None,
        from_ledger: int | None = None,
        to_ledger: int | None = None,
    ) -> AsyncIterator[Any]:
        """Yield every invocation across all cursor pages."""
        cursor: str | None = None
        while True:
            page = await self.list(
                contract_id,
                cursor=cursor,
                limit=limit,
                network=network,
                status=status,
                fn=fn,
                from_ledger=from_ledger,
                to_ledger=to_ledger,
            )
            for invocation in page.invocations:
                yield invocation
            if not page.next_cursor:
                return
            cursor = page.next_cursor

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


class _AsyncAlerts(_AsyncResource):
    async def list(
        self,
        *,
        flat: bool | None = None,
        contract_id: str | None = None,
        severity: str | None = None,
        network: str | None = None,
        cursor: str | None = None,
        limit: int | None = None,
    ) -> ListAlertsResponse200:
        """List alert groups; pass ``flat=True`` for the raw per-event feed."""
        return _unwrap(
            await list_alerts.asyncio_detailed(
                client=self._client,
                flat=_value(flat),
                contract_id=_value(contract_id),
                severity=_enum(ListAlertsSeverity, severity),
                network=_enum(ListAlertsNetwork, network),
                cursor=_value(cursor),
                limit=_value(limit),
            )
        )


class _AsyncSearch(_AsyncResource):
    async def contracts(self, q: str, *, limit: int = 10) -> GetApiV1SearchResponse200:
        """Search tracked contracts by ID or label."""
        return _unwrap(
            await get_api_v1_search.asyncio_detailed(client=self._client, q=q, limit=limit)
        )


class _AsyncLabels(_AsyncResource):
    async def list(self, *, query: str | None = None) -> Any:
        """Search visible account labels, optionally narrowed by ``query``.

        Returns the decoded JSON body (the operation declares no response
        schema in ``docs/openapi.yaml``).
        """
        return _payload(
            await get_api_v1_labels.asyncio_detailed(
                client=self._client, query=_value(query)
            )
        )

    async def create(self, label: str, value: str, *, scope: str = "public") -> Any:
        """Create or update a label; ``scope`` is ``"public"`` or ``"workspace"``."""
        body = PostApiV1LabelsBody(
            label=label,
            value=value,
            scope=PostApiV1LabelsBodyScope(scope),
        )
        return _payload(
            await post_api_v1_labels.asyncio_detailed(client=self._client, body=body)
        )

    async def resolve(self, query: str) -> Any:
        """Resolve a visible label to its Stellar identifier."""
        return _payload(
            await get_api_v1_resolve.asyncio_detailed(client=self._client, query=query)
        )


class _AsyncDlq(_AsyncResource):
    async def list(
        self, *, cursor: str | None = None, limit: int | None = None
    ) -> ListFailedEventsResponse200:
        """List dead-lettered indexer events, newest first."""
        return _unwrap(
            await list_failed_events.asyncio_detailed(
                client=self._client, cursor=_value(cursor), limit=_value(limit)
            )
        )

    async def requeue(self, event_id: int) -> RequeueFailedEventResponse200:
        """Requeue a dead-lettered event by its DLQ row id."""
        return _unwrap(
            await requeue_failed_event.asyncio_detailed(event_id, client=self._client)
        )


class _AsyncReports(_AsyncResource):
    async def contract(
        self,
        contract_id: str,
        *,
        month: str | None = None,
        format_: str | None = None,
    ) -> ContractReport:
        """Monthly SLA/uptime report; ``format_`` is ``json``, ``csv`` or ``pdf``."""
        return _unwrap(
            await get_contract_report.asyncio_detailed(
                contract_id,
                client=self._client,
                month=_value(month),
                format_=_enum(GetContractReportFormat, format_),
            )
        )

    async def history(
        self, contract_id: str, *, months: int | None = None
    ) -> GetContractReportHistoryResponse200:
        """SLA history across the trailing ``months`` months."""
        return _unwrap(
            await get_contract_report_history.asyncio_detailed(
                contract_id, client=self._client, months=_value(months)
            )
        )

    async def badge(self, contract_id: str, *, month: str | None = None) -> str:
        """Shields.io-style SVG badge for a month's uptime."""
        response = await get_contract_sla_badge.asyncio_detailed(
            contract_id, client=self._client, month=_value(month)
        )
        exceptions.raise_for_response(response.status_code, response.parsed, response.content)
        return response.content.decode("utf-8")

    async def create_subscription(
        self, email: str, frequency: str, *, day_of_week: int | None = None
    ) -> ReportSubscription:
        """Subscribe an email to daily or weekly digest reports."""
        body = CreateReportSubscriptionBody(
            email=email,
            frequency=CreateReportSubscriptionBodyFrequency(frequency),
            day_of_week=_value(day_of_week),
        )
        return _unwrap(
            await create_report_subscription.asyncio_detailed(client=self._client, body=body)
        )

    async def subscriptions(self, email: str) -> ListReportSubscriptionsResponse200:
        """Active digest subscriptions for an email address."""
        return _unwrap(
            await list_report_subscriptions.asyncio_detailed(
                client=self._client, email=email
            )
        )

    async def delete_subscription(self, subscription_id: str) -> None:
        """Delete a digest subscription."""
        _unwrap(
            await delete_report_subscription.asyncio_detailed(
                subscription_id, client=self._client
            )
        )

    async def unsubscribe(self, token: str) -> str:
        """One-click unsubscribe from digest emails."""
        return _unwrap(
            await unsubscribe_report.asyncio_detailed(client=self._client, token=token)
        )


class _AsyncNotifications(_AsyncResource):
    async def list(self) -> ListAlertSubscriptionsResponse200:
        """List alert notification subscriptions, newest first."""
        return _unwrap(await list_alert_subscriptions.asyncio_detailed(client=self._client))

    async def create(
        self,
        contract_id: str,
        *,
        channel_type: str = "webhook",
        webhook_url: str | None = None,
        routing_key: str | None = None,
        severity_filter: str = "Critical",
    ) -> AlertSubscription:
        """Subscribe a channel (webhook/slack/discord/pagerduty) to alerts."""
        body = CreateAlertSubscription(
            contract_id=contract_id,
            channel_type=CreateAlertSubscriptionChannelType(channel_type),
            webhook_url=_value(webhook_url),
            routing_key=_value(routing_key),
            severity_filter=CreateAlertSubscriptionSeverityFilter(severity_filter),
        )
        return _unwrap(
            await create_alert_subscription.asyncio_detailed(client=self._client, body=body)
        )

    async def delete(self, subscription_id: str) -> None:
        """Delete an alert subscription."""
        _unwrap(
            await delete_alert_subscription.asyncio_detailed(
                subscription_id, client=self._client
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
        retry: RetryPolicy | bool | None = True,
        **httpx_args: Any,
    ) -> None:
        self._client = _build_client(
            api_key=api_key,
            base_url=base_url,
            timeout=timeout,
            headers=headers,
            retry=retry,
            **httpx_args,
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
        self.alerts = _AsyncAlerts(self)
        self.search = _AsyncSearch(self)
        self.labels = _AsyncLabels(self)
        self.dlq = _AsyncDlq(self)
        self.reports = _AsyncReports(self)
        self.notifications = _AsyncNotifications(self)

    async def close(self) -> None:
        await self._client.get_async_httpx_client().aclose()

    async def __aenter__(self) -> AsyncClient:
        await self._client.get_async_httpx_client().__aenter__()
        return self

    async def __aexit__(self, *exc_info: Any) -> None:
        await self._client.get_async_httpx_client().__aexit__(*exc_info)
