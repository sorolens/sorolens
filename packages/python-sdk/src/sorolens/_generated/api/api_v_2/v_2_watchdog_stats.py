from http import HTTPStatus
from typing import Any

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.v2_watchdog_stats import V2WatchdogStats
from ...models.v2_watchdog_stats_network import V2WatchdogStatsNetwork
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    network: V2WatchdogStatsNetwork | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    json_network: str | Unset = UNSET
    if not isinstance(network, Unset):
        json_network = network.value

    params["network"] = json_network

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v2/watchdog/stats",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | V2WatchdogStats | None:
    if response.status_code == 200:
        response_200 = V2WatchdogStats.from_dict(response.json())

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
) -> Response[Error | V2WatchdogStats]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient,
    network: V2WatchdogStatsNetwork | Unset = UNSET,
) -> Response[Error | V2WatchdogStats]:
    """Watchdog monitoring summary

    Args:
        network (V2WatchdogStatsNetwork | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | V2WatchdogStats]
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
    network: V2WatchdogStatsNetwork | Unset = UNSET,
) -> Error | V2WatchdogStats | None:
    """Watchdog monitoring summary

    Args:
        network (V2WatchdogStatsNetwork | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | V2WatchdogStats
    """

    return sync_detailed(
        client=client,
        network=network,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient,
    network: V2WatchdogStatsNetwork | Unset = UNSET,
) -> Response[Error | V2WatchdogStats]:
    """Watchdog monitoring summary

    Args:
        network (V2WatchdogStatsNetwork | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | V2WatchdogStats]
    """

    kwargs = _get_kwargs(
        network=network,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient,
    network: V2WatchdogStatsNetwork | Unset = UNSET,
) -> Error | V2WatchdogStats | None:
    """Watchdog monitoring summary

    Args:
        network (V2WatchdogStatsNetwork | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | V2WatchdogStats
    """

    return (
        await asyncio_detailed(
            client=client,
            network=network,
        )
    ).parsed
