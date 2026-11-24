from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.list_token_transfers_network import ListTokenTransfersNetwork
from ...models.list_token_transfers_response_200 import ListTokenTransfersResponse200
from ...models.list_token_transfers_type import ListTokenTransfersType
from ...types import UNSET, Response, Unset


def _get_kwargs(
    id: str,
    *,
    cursor: str | Unset = UNSET,
    network: ListTokenTransfersNetwork | Unset = UNSET,
    type_: ListTokenTransfersType | Unset = UNSET,
    from_: int | Unset = UNSET,
    to: int | Unset = UNSET,
    limit: int | Unset = 50,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["cursor"] = cursor

    json_network: str | Unset = UNSET
    if not isinstance(network, Unset):
        json_network = network.value

    params["network"] = json_network

    json_type_: str | Unset = UNSET
    if not isinstance(type_, Unset):
        json_type_ = type_.value

    params["type"] = json_type_

    params["from"] = from_

    params["to"] = to

    params["limit"] = limit

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v1/contracts/{id}/transfers".format(
            id=quote(str(id), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | ListTokenTransfersResponse200 | None:
    if response.status_code == 200:
        response_200 = ListTokenTransfersResponse200.from_dict(response.json())

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
) -> Response[Error | ListTokenTransfersResponse200]:
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
    cursor: str | Unset = UNSET,
    network: ListTokenTransfersNetwork | Unset = UNSET,
    type_: ListTokenTransfersType | Unset = UNSET,
    from_: int | Unset = UNSET,
    to: int | Unset = UNSET,
    limit: int | Unset = 50,
) -> Response[Error | ListTokenTransfersResponse200]:
    """List SEP-41 token transfers

     Returns paginated SEP-41 token transfer events for a contract (transfer, mint, burn, clawback,
    approve), newest first.

    Args:
        id (str):
        cursor (str | Unset):
        network (ListTokenTransfersNetwork | Unset):
        type_ (ListTokenTransfersType | Unset):
        from_ (int | Unset):
        to (int | Unset):
        limit (int | Unset):  Default: 50.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | ListTokenTransfersResponse200]
    """

    kwargs = _get_kwargs(
        id=id,
        cursor=cursor,
        network=network,
        type_=type_,
        from_=from_,
        to=to,
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
    cursor: str | Unset = UNSET,
    network: ListTokenTransfersNetwork | Unset = UNSET,
    type_: ListTokenTransfersType | Unset = UNSET,
    from_: int | Unset = UNSET,
    to: int | Unset = UNSET,
    limit: int | Unset = 50,
) -> Error | ListTokenTransfersResponse200 | None:
    """List SEP-41 token transfers

     Returns paginated SEP-41 token transfer events for a contract (transfer, mint, burn, clawback,
    approve), newest first.

    Args:
        id (str):
        cursor (str | Unset):
        network (ListTokenTransfersNetwork | Unset):
        type_ (ListTokenTransfersType | Unset):
        from_ (int | Unset):
        to (int | Unset):
        limit (int | Unset):  Default: 50.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | ListTokenTransfersResponse200
    """

    return sync_detailed(
        id=id,
        client=client,
        cursor=cursor,
        network=network,
        type_=type_,
        from_=from_,
        to=to,
        limit=limit,
    ).parsed


async def asyncio_detailed(
    id: str,
    *,
    client: AuthenticatedClient,
    cursor: str | Unset = UNSET,
    network: ListTokenTransfersNetwork | Unset = UNSET,
    type_: ListTokenTransfersType | Unset = UNSET,
    from_: int | Unset = UNSET,
    to: int | Unset = UNSET,
    limit: int | Unset = 50,
) -> Response[Error | ListTokenTransfersResponse200]:
    """List SEP-41 token transfers

     Returns paginated SEP-41 token transfer events for a contract (transfer, mint, burn, clawback,
    approve), newest first.

    Args:
        id (str):
        cursor (str | Unset):
        network (ListTokenTransfersNetwork | Unset):
        type_ (ListTokenTransfersType | Unset):
        from_ (int | Unset):
        to (int | Unset):
        limit (int | Unset):  Default: 50.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | ListTokenTransfersResponse200]
    """

    kwargs = _get_kwargs(
        id=id,
        cursor=cursor,
        network=network,
        type_=type_,
        from_=from_,
        to=to,
        limit=limit,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    id: str,
    *,
    client: AuthenticatedClient,
    cursor: str | Unset = UNSET,
    network: ListTokenTransfersNetwork | Unset = UNSET,
    type_: ListTokenTransfersType | Unset = UNSET,
    from_: int | Unset = UNSET,
    to: int | Unset = UNSET,
    limit: int | Unset = 50,
) -> Error | ListTokenTransfersResponse200 | None:
    """List SEP-41 token transfers

     Returns paginated SEP-41 token transfer events for a contract (transfer, mint, burn, clawback,
    approve), newest first.

    Args:
        id (str):
        cursor (str | Unset):
        network (ListTokenTransfersNetwork | Unset):
        type_ (ListTokenTransfersType | Unset):
        from_ (int | Unset):
        to (int | Unset):
        limit (int | Unset):  Default: 50.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | ListTokenTransfersResponse200
    """

    return (
        await asyncio_detailed(
            id=id,
            client=client,
            cursor=cursor,
            network=network,
            type_=type_,
            from_=from_,
            to=to,
            limit=limit,
        )
    ).parsed
