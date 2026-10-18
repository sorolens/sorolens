from http import HTTPStatus
from typing import Any

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.v2_contract_list import V2ContractList
from ...models.v2_list_contracts_network import V2ListContractsNetwork
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    network: V2ListContractsNetwork | Unset = UNSET,
    status: str | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["cursor"] = cursor

    params["limit"] = limit

    json_network: str | Unset = UNSET
    if not isinstance(network, Unset):
        json_network = network.value

    params["network"] = json_network

    params["status"] = status

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v2/contracts",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | V2ContractList | None:
    if response.status_code == 200:
        response_200 = V2ContractList.from_dict(response.json())

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
) -> Response[Error | V2ContractList]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    network: V2ListContractsNetwork | Unset = UNSET,
    status: str | Unset = UNSET,
) -> Response[Error | V2ContractList]:
    """List tracked contracts

    Args:
        cursor (str | Unset):
        limit (int | Unset):
        network (V2ListContractsNetwork | Unset):
        status (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | V2ContractList]
    """

    kwargs = _get_kwargs(
        cursor=cursor,
        limit=limit,
        network=network,
        status=status,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    network: V2ListContractsNetwork | Unset = UNSET,
    status: str | Unset = UNSET,
) -> Error | V2ContractList | None:
    """List tracked contracts

    Args:
        cursor (str | Unset):
        limit (int | Unset):
        network (V2ListContractsNetwork | Unset):
        status (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | V2ContractList
    """

    return sync_detailed(
        client=client,
        cursor=cursor,
        limit=limit,
        network=network,
        status=status,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    network: V2ListContractsNetwork | Unset = UNSET,
    status: str | Unset = UNSET,
) -> Response[Error | V2ContractList]:
    """List tracked contracts

    Args:
        cursor (str | Unset):
        limit (int | Unset):
        network (V2ListContractsNetwork | Unset):
        status (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | V2ContractList]
    """

    kwargs = _get_kwargs(
        cursor=cursor,
        limit=limit,
        network=network,
        status=status,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    network: V2ListContractsNetwork | Unset = UNSET,
    status: str | Unset = UNSET,
) -> Error | V2ContractList | None:
    """List tracked contracts

    Args:
        cursor (str | Unset):
        limit (int | Unset):
        network (V2ListContractsNetwork | Unset):
        status (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | V2ContractList
    """

    return (
        await asyncio_detailed(
            client=client,
            cursor=cursor,
            limit=limit,
            network=network,
            status=status,
        )
    ).parsed
