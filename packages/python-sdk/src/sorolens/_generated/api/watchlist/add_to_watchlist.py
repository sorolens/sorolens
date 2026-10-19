from http import HTTPStatus
from typing import Any

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.add_to_watchlist_body import AddToWatchlistBody
from ...models.error import Error
from ...models.in_watchlist import InWatchlist
from ...types import Response


def _get_kwargs(
    *,
    body: AddToWatchlistBody,
    x_user_id: str,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    headers["X-User-ID"] = x_user_id

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/api/v1/watchlist",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | InWatchlist | None:
    if response.status_code == 201:
        response_201 = InWatchlist.from_dict(response.json())

        return response_201

    if response.status_code == 401:
        response_401 = Error.from_dict(response.json())

        return response_401

    if response.status_code == 415:
        response_415 = Error.from_dict(response.json())

        return response_415

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
) -> Response[Error | InWatchlist]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient,
    body: AddToWatchlistBody,
    x_user_id: str,
) -> Response[Error | InWatchlist]:
    """Add a contract to the caller's watchlist

    Args:
        x_user_id (str):
        body (AddToWatchlistBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | InWatchlist]
    """

    kwargs = _get_kwargs(
        body=body,
        x_user_id=x_user_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient,
    body: AddToWatchlistBody,
    x_user_id: str,
) -> Error | InWatchlist | None:
    """Add a contract to the caller's watchlist

    Args:
        x_user_id (str):
        body (AddToWatchlistBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | InWatchlist
    """

    return sync_detailed(
        client=client,
        body=body,
        x_user_id=x_user_id,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient,
    body: AddToWatchlistBody,
    x_user_id: str,
) -> Response[Error | InWatchlist]:
    """Add a contract to the caller's watchlist

    Args:
        x_user_id (str):
        body (AddToWatchlistBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | InWatchlist]
    """

    kwargs = _get_kwargs(
        body=body,
        x_user_id=x_user_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient,
    body: AddToWatchlistBody,
    x_user_id: str,
) -> Error | InWatchlist | None:
    """Add a contract to the caller's watchlist

    Args:
        x_user_id (str):
        body (AddToWatchlistBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | InWatchlist
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
            x_user_id=x_user_id,
        )
    ).parsed
