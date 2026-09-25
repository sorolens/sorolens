from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.error import Error
from ...models.in_watchlist import InWatchlist
from typing import cast



def _get_kwargs(
    contract_id: str,
    *,
    x_user_id: str,

) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    headers["X-User-ID"] = x_user_id



    

    

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v1/watchlist/{contract_id}/status".format(contract_id=quote(str(contract_id), safe=""),),
    }


    _kwargs["headers"] = headers
    return _kwargs



def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Error | InWatchlist | None:
    if response.status_code == 200:
        response_200 = InWatchlist.from_dict(response.json())



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


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[Error | InWatchlist]:
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
    x_user_id: str,

) -> Response[Error | InWatchlist]:
    """ Check watchlist membership for a contract

    Args:
        contract_id (str):
        x_user_id (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | InWatchlist]
     """


    kwargs = _get_kwargs(
        contract_id=contract_id,
x_user_id=x_user_id,

    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)

def sync(
    contract_id: str,
    *,
    client: AuthenticatedClient,
    x_user_id: str,

) -> Error | InWatchlist | None:
    """ Check watchlist membership for a contract

    Args:
        contract_id (str):
        x_user_id (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | InWatchlist
     """


    return sync_detailed(
        contract_id=contract_id,
client=client,
x_user_id=x_user_id,

    ).parsed

async def asyncio_detailed(
    contract_id: str,
    *,
    client: AuthenticatedClient,
    x_user_id: str,

) -> Response[Error | InWatchlist]:
    """ Check watchlist membership for a contract

    Args:
        contract_id (str):
        x_user_id (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | InWatchlist]
     """


    kwargs = _get_kwargs(
        contract_id=contract_id,
x_user_id=x_user_id,

    )

    response = await client.get_async_httpx_client().request(
        **kwargs
    )

    return _build_response(client=client, response=response)

async def asyncio(
    contract_id: str,
    *,
    client: AuthenticatedClient,
    x_user_id: str,

) -> Error | InWatchlist | None:
    """ Check watchlist membership for a contract

    Args:
        contract_id (str):
        x_user_id (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | InWatchlist
     """


    return (await asyncio_detailed(
        contract_id=contract_id,
client=client,
x_user_id=x_user_id,

    )).parsed
