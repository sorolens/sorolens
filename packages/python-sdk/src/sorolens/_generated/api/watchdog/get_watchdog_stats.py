from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.error import Error
from ...models.get_watchdog_stats_network import GetWatchdogStatsNetwork
from ...models.watchdog_stats import WatchdogStats
from ...types import UNSET, Unset
from typing import cast



def _get_kwargs(
    *,
    network: GetWatchdogStatsNetwork | Unset = UNSET,

) -> dict[str, Any]:
    

    

    params: dict[str, Any] = {}

    json_network: str | Unset = UNSET
    if not isinstance(network, Unset):
        json_network = network.value

    params["network"] = json_network


    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}


    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v1/watchdog/stats",
        "params": params,
    }


    return _kwargs



def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Error | WatchdogStats | None:
    if response.status_code == 200:
        response_200 = WatchdogStats.from_dict(response.json())



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


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[Error | WatchdogStats]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient,
    network: GetWatchdogStatsNetwork | Unset = UNSET,

) -> Response[Error | WatchdogStats]:
    """ Watchdog statistics

    Args:
        network (GetWatchdogStatsNetwork | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | WatchdogStats]
     """


    kwargs = _get_kwargs(
        network=network,

    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)

def sync(
    *,
    client: AuthenticatedClient,
    network: GetWatchdogStatsNetwork | Unset = UNSET,

) -> Error | WatchdogStats | None:
    """ Watchdog statistics

    Args:
        network (GetWatchdogStatsNetwork | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | WatchdogStats
     """


    return sync_detailed(
        client=client,
network=network,

    ).parsed

async def asyncio_detailed(
    *,
    client: AuthenticatedClient,
    network: GetWatchdogStatsNetwork | Unset = UNSET,

) -> Response[Error | WatchdogStats]:
    """ Watchdog statistics

    Args:
        network (GetWatchdogStatsNetwork | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | WatchdogStats]
     """


    kwargs = _get_kwargs(
        network=network,

    )

    response = await client.get_async_httpx_client().request(
        **kwargs
    )

    return _build_response(client=client, response=response)

async def asyncio(
    *,
    client: AuthenticatedClient,
    network: GetWatchdogStatsNetwork | Unset = UNSET,

) -> Error | WatchdogStats | None:
    """ Watchdog statistics

    Args:
        network (GetWatchdogStatsNetwork | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | WatchdogStats
     """


    return (await asyncio_detailed(
        client=client,
network=network,

    )).parsed
