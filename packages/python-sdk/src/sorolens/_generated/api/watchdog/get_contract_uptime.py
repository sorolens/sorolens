from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.get_contract_uptime_window import GetContractUptimeWindow
from ...models.uptime_result import UptimeResult
from ...types import UNSET, Response, Unset


def _get_kwargs(
    id: str,
    *,
    window: GetContractUptimeWindow | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    json_window: str | Unset = UNSET
    if not isinstance(window, Unset):
        json_window = window.value

    params["window"] = json_window

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v1/watchdog/contracts/{id}/uptime".format(
            id=quote(str(id), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | UptimeResult | None:
    if response.status_code == 200:
        response_200 = UptimeResult.from_dict(response.json())

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


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Error | UptimeResult]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    id: str,
    *,
    client: AuthenticatedClient,
    window: GetContractUptimeWindow | Unset = UNSET,
) -> Response[Error | UptimeResult]:
    """Get uptime percentage for a monitored contract

    Args:
        id (str):
        window (GetContractUptimeWindow | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | UptimeResult]
    """

    kwargs = _get_kwargs(
        id=id,
        window=window,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    id: str,
    *,
    client: AuthenticatedClient,
    window: GetContractUptimeWindow | Unset = UNSET,
) -> Error | UptimeResult | None:
    """Get uptime percentage for a monitored contract

    Args:
        id (str):
        window (GetContractUptimeWindow | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | UptimeResult
    """

    return sync_detailed(
        id=id,
        client=client,
        window=window,
    ).parsed


async def asyncio_detailed(
    id: str,
    *,
    client: AuthenticatedClient,
    window: GetContractUptimeWindow | Unset = UNSET,
) -> Response[Error | UptimeResult]:
    """Get uptime percentage for a monitored contract

    Args:
        id (str):
        window (GetContractUptimeWindow | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | UptimeResult]
    """

    kwargs = _get_kwargs(
        id=id,
        window=window,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    id: str,
    *,
    client: AuthenticatedClient,
    window: GetContractUptimeWindow | Unset = UNSET,
) -> Error | UptimeResult | None:
    """Get uptime percentage for a monitored contract

    Args:
        id (str):
        window (GetContractUptimeWindow | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | UptimeResult
    """

    return (
        await asyncio_detailed(
            id=id,
            client=client,
            window=window,
        )
    ).parsed
