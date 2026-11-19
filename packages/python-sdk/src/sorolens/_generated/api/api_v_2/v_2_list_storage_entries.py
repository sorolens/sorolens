from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.v2_list_storage_entries_durability import V2ListStorageEntriesDurability
from ...models.v2_list_storage_entries_network import V2ListStorageEntriesNetwork
from ...models.v2_list_storage_entries_status import V2ListStorageEntriesStatus
from ...models.v2_storage_list import V2StorageList
from ...types import UNSET, Response, Unset


def _get_kwargs(
    id: str,
    *,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    network: V2ListStorageEntriesNetwork | Unset = UNSET,
    durability: V2ListStorageEntriesDurability | Unset = UNSET,
    status: V2ListStorageEntriesStatus | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["cursor"] = cursor

    params["limit"] = limit

    json_network: str | Unset = UNSET
    if not isinstance(network, Unset):
        json_network = network.value

    params["network"] = json_network

    json_durability: str | Unset = UNSET
    if not isinstance(durability, Unset):
        json_durability = durability.value

    params["durability"] = json_durability

    json_status: str | Unset = UNSET
    if not isinstance(status, Unset):
        json_status = status.value

    params["status"] = json_status

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v2/contracts/{id}/storage".format(
            id=quote(str(id), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | V2StorageList | None:
    if response.status_code == 200:
        response_200 = V2StorageList.from_dict(response.json())

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
) -> Response[Error | V2StorageList]:
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
    limit: int | Unset = UNSET,
    network: V2ListStorageEntriesNetwork | Unset = UNSET,
    durability: V2ListStorageEntriesDurability | Unset = UNSET,
    status: V2ListStorageEntriesStatus | Unset = UNSET,
) -> Response[Error | V2StorageList]:
    """List a contract's storage entries

    Args:
        id (str):
        cursor (str | Unset):
        limit (int | Unset):
        network (V2ListStorageEntriesNetwork | Unset):
        durability (V2ListStorageEntriesDurability | Unset):
        status (V2ListStorageEntriesStatus | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | V2StorageList]
    """

    kwargs = _get_kwargs(
        id=id,
        cursor=cursor,
        limit=limit,
        network=network,
        durability=durability,
        status=status,
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
    limit: int | Unset = UNSET,
    network: V2ListStorageEntriesNetwork | Unset = UNSET,
    durability: V2ListStorageEntriesDurability | Unset = UNSET,
    status: V2ListStorageEntriesStatus | Unset = UNSET,
) -> Error | V2StorageList | None:
    """List a contract's storage entries

    Args:
        id (str):
        cursor (str | Unset):
        limit (int | Unset):
        network (V2ListStorageEntriesNetwork | Unset):
        durability (V2ListStorageEntriesDurability | Unset):
        status (V2ListStorageEntriesStatus | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | V2StorageList
    """

    return sync_detailed(
        id=id,
        client=client,
        cursor=cursor,
        limit=limit,
        network=network,
        durability=durability,
        status=status,
    ).parsed


async def asyncio_detailed(
    id: str,
    *,
    client: AuthenticatedClient,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    network: V2ListStorageEntriesNetwork | Unset = UNSET,
    durability: V2ListStorageEntriesDurability | Unset = UNSET,
    status: V2ListStorageEntriesStatus | Unset = UNSET,
) -> Response[Error | V2StorageList]:
    """List a contract's storage entries

    Args:
        id (str):
        cursor (str | Unset):
        limit (int | Unset):
        network (V2ListStorageEntriesNetwork | Unset):
        durability (V2ListStorageEntriesDurability | Unset):
        status (V2ListStorageEntriesStatus | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | V2StorageList]
    """

    kwargs = _get_kwargs(
        id=id,
        cursor=cursor,
        limit=limit,
        network=network,
        durability=durability,
        status=status,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    id: str,
    *,
    client: AuthenticatedClient,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    network: V2ListStorageEntriesNetwork | Unset = UNSET,
    durability: V2ListStorageEntriesDurability | Unset = UNSET,
    status: V2ListStorageEntriesStatus | Unset = UNSET,
) -> Error | V2StorageList | None:
    """List a contract's storage entries

    Args:
        id (str):
        cursor (str | Unset):
        limit (int | Unset):
        network (V2ListStorageEntriesNetwork | Unset):
        durability (V2ListStorageEntriesDurability | Unset):
        status (V2ListStorageEntriesStatus | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | V2StorageList
    """

    return (
        await asyncio_detailed(
            id=id,
            client=client,
            cursor=cursor,
            limit=limit,
            network=network,
            durability=durability,
            status=status,
        )
    ).parsed
