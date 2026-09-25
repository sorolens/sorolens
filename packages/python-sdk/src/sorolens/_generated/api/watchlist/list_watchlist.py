from http import HTTPStatus
from typing import Any

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.watchlist import Watchlist
from ...types import Response


def _get_kwargs(
    *,
    x_user_id: str,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    headers["X-User-ID"] = x_user_id

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v1/watchlist",
    }

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | Watchlist | None:
    if response.status_code == 200:
        response_200 = Watchlist.from_dict(response.json())

        return response_200

    if response.status_code == 401:
        response_401 = Error.from_dict(response.json())

        return response_401

    if response.status_code == 500:
        response_500 = Error.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Error | Watchlist]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient,
    x_user_id: str,
) -> Response[Error | Watchlist]:
    """List the caller's watchlist

    Args:
        x_user_id (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | Watchlist]
    """

    kwargs = _get_kwargs(
        x_user_id=x_user_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient,
    x_user_id: str,
) -> Error | Watchlist | None:
    """List the caller's watchlist

    Args:
        x_user_id (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | Watchlist
    """

    return sync_detailed(
        client=client,
        x_user_id=x_user_id,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient,
    x_user_id: str,
) -> Response[Error | Watchlist]:
    """List the caller's watchlist

    Args:
        x_user_id (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | Watchlist]
    """

    kwargs = _get_kwargs(
        x_user_id=x_user_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient,
    x_user_id: str,
) -> Error | Watchlist | None:
    """List the caller's watchlist

    Args:
        x_user_id (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | Watchlist
    """

    return (
        await asyncio_detailed(
            client=client,
            x_user_id=x_user_id,
        )
    ).parsed
