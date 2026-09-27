from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.v2_alert_list import V2AlertList
from ...types import UNSET, Response, Unset


def _get_kwargs(
    id: str,
    *,
    limit: int | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["limit"] = limit

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v2/watchdog/contracts/{id}/alerts".format(
            id=quote(str(id), safe=""),
        ),
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
    id: str,
    *,
    client: AuthenticatedClient,
    limit: int | Unset = UNSET,
) -> Response[Error | V2AlertList]:
    """Alerts for one monitored contract

    Args:
        id (str):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | V2AlertList]
    """

    kwargs = _get_kwargs(
        id=id,
        limit=limit,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    id: str,
    *,
    client: AuthenticatedClient,
    limit: int | Unset = UNSET,
) -> Error | V2AlertList | None:
    """Alerts for one monitored contract

    Args:
        id (str):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | V2AlertList
    """

    return sync_detailed(
        id=id,
        client=client,
        limit=limit,
    ).parsed


async def asyncio_detailed(
    id: str,
    *,
    client: AuthenticatedClient,
    limit: int | Unset = UNSET,
) -> Response[Error | V2AlertList]:
    """Alerts for one monitored contract

    Args:
        id (str):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | V2AlertList]
    """

    kwargs = _get_kwargs(
        id=id,
        limit=limit,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    id: str,
    *,
    client: AuthenticatedClient,
    limit: int | Unset = UNSET,
) -> Error | V2AlertList | None:
    """Alerts for one monitored contract

    Args:
        id (str):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | V2AlertList
    """

    return (
        await asyncio_detailed(
            id=id,
            client=client,
            limit=limit,
        )
    ).parsed
