from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.list_token_holders_network import ListTokenHoldersNetwork
from ...models.list_token_holders_response_200 import ListTokenHoldersResponse200
from ...types import UNSET, Response, Unset


def _get_kwargs(
    id: str,
    *,
    network: ListTokenHoldersNetwork | Unset = UNSET,
    cursor: str | Unset = UNSET,
    limit: int | Unset = 50,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    json_network: str | Unset = UNSET
    if not isinstance(network, Unset):
        json_network = network.value

    params["network"] = json_network

    params["cursor"] = cursor

    params["limit"] = limit

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v1/contracts/{id}/holders".format(
            id=quote(str(id), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | ListTokenHoldersResponse200 | None:
    if response.status_code == 200:
        response_200 = ListTokenHoldersResponse200.from_dict(response.json())

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
) -> Response[Error | ListTokenHoldersResponse200]:
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
    network: ListTokenHoldersNetwork | Unset = UNSET,
    cursor: str | Unset = UNSET,
    limit: int | Unset = 50,
) -> Response[Error | ListTokenHoldersResponse200]:
    """List SEP-41 token holders

     Returns token holders for a contract sorted by balance descending. Balances are aggregated from
    indexed transfer, mint and burn events.

    Args:
        id (str):
        network (ListTokenHoldersNetwork | Unset):
        cursor (str | Unset):
        limit (int | Unset):  Default: 50.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | ListTokenHoldersResponse200]
    """

    kwargs = _get_kwargs(
        id=id,
        network=network,
        cursor=cursor,
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
    network: ListTokenHoldersNetwork | Unset = UNSET,
    cursor: str | Unset = UNSET,
    limit: int | Unset = 50,
) -> Error | ListTokenHoldersResponse200 | None:
    """List SEP-41 token holders

     Returns token holders for a contract sorted by balance descending. Balances are aggregated from
    indexed transfer, mint and burn events.

    Args:
        id (str):
        network (ListTokenHoldersNetwork | Unset):
        cursor (str | Unset):
        limit (int | Unset):  Default: 50.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | ListTokenHoldersResponse200
    """

    return sync_detailed(
        id=id,
        client=client,
        network=network,
        cursor=cursor,
        limit=limit,
    ).parsed


async def asyncio_detailed(
    id: str,
    *,
    client: AuthenticatedClient,
    network: ListTokenHoldersNetwork | Unset = UNSET,
    cursor: str | Unset = UNSET,
    limit: int | Unset = 50,
) -> Response[Error | ListTokenHoldersResponse200]:
    """List SEP-41 token holders

     Returns token holders for a contract sorted by balance descending. Balances are aggregated from
    indexed transfer, mint and burn events.

    Args:
        id (str):
        network (ListTokenHoldersNetwork | Unset):
        cursor (str | Unset):
        limit (int | Unset):  Default: 50.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | ListTokenHoldersResponse200]
    """

    kwargs = _get_kwargs(
        id=id,
        network=network,
        cursor=cursor,
        limit=limit,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    id: str,
    *,
    client: AuthenticatedClient,
    network: ListTokenHoldersNetwork | Unset = UNSET,
    cursor: str | Unset = UNSET,
    limit: int | Unset = 50,
) -> Error | ListTokenHoldersResponse200 | None:
    """List SEP-41 token holders

     Returns token holders for a contract sorted by balance descending. Balances are aggregated from
    indexed transfer, mint and burn events.

    Args:
        id (str):
        network (ListTokenHoldersNetwork | Unset):
        cursor (str | Unset):
        limit (int | Unset):  Default: 50.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | ListTokenHoldersResponse200
    """

    return (
        await asyncio_detailed(
            id=id,
            client=client,
            network=network,
            cursor=cursor,
            limit=limit,
        )
    ).parsed
