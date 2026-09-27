from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.v2_watchlist_status import V2WatchlistStatus
from ...types import Response


def _get_kwargs(
    contract_id: str,
) -> dict[str, Any]:

    _kwargs: dict[str, Any] = {
        "method": "delete",
        "url": "/api/v2/watchlist/{contract_id}".format(
            contract_id=quote(str(contract_id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | V2WatchlistStatus | None:
    if response.status_code == 200:
        response_200 = V2WatchlistStatus.from_dict(response.json())

        return response_200

    if response.status_code == 401:
        response_401 = Error.from_dict(response.json())

        return response_401

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
) -> Response[Error | V2WatchlistStatus]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    contract_id: str,
    *,
    client: AuthenticatedClient,
) -> Response[Error | V2WatchlistStatus]:
    """Remove a contract from the caller's watchlist

    Args:
        contract_id (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | V2WatchlistStatus]
    """

    kwargs = _get_kwargs(
        contract_id=contract_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    contract_id: str,
    *,
    client: AuthenticatedClient,
) -> Error | V2WatchlistStatus | None:
    """Remove a contract from the caller's watchlist

    Args:
        contract_id (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | V2WatchlistStatus
    """

    return sync_detailed(
        contract_id=contract_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    contract_id: str,
    *,
    client: AuthenticatedClient,
) -> Response[Error | V2WatchlistStatus]:
    """Remove a contract from the caller's watchlist

    Args:
        contract_id (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | V2WatchlistStatus]
    """

    kwargs = _get_kwargs(
        contract_id=contract_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    contract_id: str,
    *,
    client: AuthenticatedClient,
) -> Error | V2WatchlistStatus | None:
    """Remove a contract from the caller's watchlist

    Args:
        contract_id (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | V2WatchlistStatus
    """

    return (
        await asyncio_detailed(
            contract_id=contract_id,
            client=client,
        )
    ).parsed
