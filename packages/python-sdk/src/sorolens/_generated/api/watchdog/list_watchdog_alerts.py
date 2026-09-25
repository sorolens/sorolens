from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.error import Error
from ...models.list_watchdog_alerts_network import ListWatchdogAlertsNetwork
from ...models.list_watchdog_alerts_response_200 import ListWatchdogAlertsResponse200
from ...models.list_watchdog_alerts_severity import ListWatchdogAlertsSeverity
from ...types import UNSET, Unset
from typing import cast



def _get_kwargs(
    *,
    severity: ListWatchdogAlertsSeverity | Unset = UNSET,
    network: ListWatchdogAlertsNetwork | Unset = UNSET,
    limit: int | Unset = UNSET,

) -> dict[str, Any]:
    

    

    params: dict[str, Any] = {}

    json_severity: str | Unset = UNSET
    if not isinstance(severity, Unset):
        json_severity = severity.value

    params["severity"] = json_severity

    json_network: str | Unset = UNSET
    if not isinstance(network, Unset):
        json_network = network.value

    params["network"] = json_network

    params["limit"] = limit


    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}


    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v1/watchdog/alerts",
        "params": params,
    }


    return _kwargs



def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Error | ListWatchdogAlertsResponse200 | None:
    if response.status_code == 200:
        response_200 = ListWatchdogAlertsResponse200.from_dict(response.json())



        return response_200

    if response.status_code == 422:
        response_422 = Error.from_dict(response.json())



        return response_422

    if response.status_code == 500:
        response_500 = Error.from_dict(response.json())



        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[Error | ListWatchdogAlertsResponse200]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient,
    severity: ListWatchdogAlertsSeverity | Unset = UNSET,
    network: ListWatchdogAlertsNetwork | Unset = UNSET,
    limit: int | Unset = UNSET,

) -> Response[Error | ListWatchdogAlertsResponse200]:
    """ List watchdog alerts

    Args:
        severity (ListWatchdogAlertsSeverity | Unset):
        network (ListWatchdogAlertsNetwork | Unset):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | ListWatchdogAlertsResponse200]
     """


    kwargs = _get_kwargs(
        severity=severity,
network=network,
limit=limit,

    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)

def sync(
    *,
    client: AuthenticatedClient,
    severity: ListWatchdogAlertsSeverity | Unset = UNSET,
    network: ListWatchdogAlertsNetwork | Unset = UNSET,
    limit: int | Unset = UNSET,

) -> Error | ListWatchdogAlertsResponse200 | None:
    """ List watchdog alerts

    Args:
        severity (ListWatchdogAlertsSeverity | Unset):
        network (ListWatchdogAlertsNetwork | Unset):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | ListWatchdogAlertsResponse200
     """


    return sync_detailed(
        client=client,
severity=severity,
network=network,
limit=limit,

    ).parsed

async def asyncio_detailed(
    *,
    client: AuthenticatedClient,
    severity: ListWatchdogAlertsSeverity | Unset = UNSET,
    network: ListWatchdogAlertsNetwork | Unset = UNSET,
    limit: int | Unset = UNSET,

) -> Response[Error | ListWatchdogAlertsResponse200]:
    """ List watchdog alerts

    Args:
        severity (ListWatchdogAlertsSeverity | Unset):
        network (ListWatchdogAlertsNetwork | Unset):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | ListWatchdogAlertsResponse200]
     """


    kwargs = _get_kwargs(
        severity=severity,
network=network,
limit=limit,

    )

    response = await client.get_async_httpx_client().request(
        **kwargs
    )

    return _build_response(client=client, response=response)

async def asyncio(
    *,
    client: AuthenticatedClient,
    severity: ListWatchdogAlertsSeverity | Unset = UNSET,
    network: ListWatchdogAlertsNetwork | Unset = UNSET,
    limit: int | Unset = UNSET,

) -> Error | ListWatchdogAlertsResponse200 | None:
    """ List watchdog alerts

    Args:
        severity (ListWatchdogAlertsSeverity | Unset):
        network (ListWatchdogAlertsNetwork | Unset):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | ListWatchdogAlertsResponse200
     """


    return (await asyncio_detailed(
        client=client,
severity=severity,
network=network,
limit=limit,

    )).parsed
