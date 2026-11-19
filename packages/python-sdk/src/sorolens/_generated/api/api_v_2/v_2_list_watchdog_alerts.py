from http import HTTPStatus
from typing import Any

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.v2_alert_list import V2AlertList
from ...models.v2_list_watchdog_alerts_network import V2ListWatchdogAlertsNetwork
from ...models.v2_list_watchdog_alerts_severity import V2ListWatchdogAlertsSeverity
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    network: V2ListWatchdogAlertsNetwork | Unset = UNSET,
    severity: V2ListWatchdogAlertsSeverity | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    json_network: str | Unset = UNSET
    if not isinstance(network, Unset):
        json_network = network.value

    params["network"] = json_network

    json_severity: str | Unset = UNSET
    if not isinstance(severity, Unset):
        json_severity = severity.value

    params["severity"] = json_severity

    params["limit"] = limit

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v2/watchdog/alerts",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | V2AlertList | None:
    if response.status_code == 200:
        response_200 = V2AlertList.from_dict(response.json())

        return response_200

    if response.status_code == 429:
        response_429 = Error.from_dict(response.json())

        return response_429

    if response.status_code == 500:
        response_500 = Error.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Error | V2AlertList]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient,
    network: V2ListWatchdogAlertsNetwork | Unset = UNSET,
    severity: V2ListWatchdogAlertsSeverity | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> Response[Error | V2AlertList]:
    """All watchdog alerts

    Args:
        network (V2ListWatchdogAlertsNetwork | Unset):
        severity (V2ListWatchdogAlertsSeverity | Unset):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | V2AlertList]
    """

    kwargs = _get_kwargs(
        network=network,
        severity=severity,
        limit=limit,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient,
    network: V2ListWatchdogAlertsNetwork | Unset = UNSET,
    severity: V2ListWatchdogAlertsSeverity | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> Error | V2AlertList | None:
    """All watchdog alerts

    Args:
        network (V2ListWatchdogAlertsNetwork | Unset):
        severity (V2ListWatchdogAlertsSeverity | Unset):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | V2AlertList
    """

    return sync_detailed(
        client=client,
        network=network,
        severity=severity,
        limit=limit,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient,
    network: V2ListWatchdogAlertsNetwork | Unset = UNSET,
    severity: V2ListWatchdogAlertsSeverity | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> Response[Error | V2AlertList]:
    """All watchdog alerts

    Args:
        network (V2ListWatchdogAlertsNetwork | Unset):
        severity (V2ListWatchdogAlertsSeverity | Unset):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | V2AlertList]
    """

    kwargs = _get_kwargs(
        network=network,
        severity=severity,
        limit=limit,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient,
    network: V2ListWatchdogAlertsNetwork | Unset = UNSET,
    severity: V2ListWatchdogAlertsSeverity | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> Error | V2AlertList | None:
    """All watchdog alerts

    Args:
        network (V2ListWatchdogAlertsNetwork | Unset):
        severity (V2ListWatchdogAlertsSeverity | Unset):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | V2AlertList
    """

    return (
        await asyncio_detailed(
            client=client,
            network=network,
            severity=severity,
            limit=limit,
        )
    ).parsed
