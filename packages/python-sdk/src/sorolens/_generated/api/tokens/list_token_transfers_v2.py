from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.list_token_transfers_v2_network import ListTokenTransfersV2Network
from ...models.list_token_transfers_v2_response_200 import (
    ListTokenTransfersV2Response200,
)
from ...types import UNSET, Response, Unset


def _get_kwargs(
    id: str,
    *,
    network: ListTokenTransfersV2Network | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    json_network: str | Unset = UNSET
    if not isinstance(network, Unset):
        json_network = network.value

    params["network"] = json_network

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v2/contracts/{id}/transfers".format(
            id=quote(str(id), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | ListTokenTransfersV2Response200 | None:
    if response.status_code == 200:
        response_200 = ListTokenTransfersV2Response200.from_dict(response.json())

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
) -> Response[Error | ListTokenTransfersV2Response200]:
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
    network: ListTokenTransfersV2Network | Unset = UNSET,
) -> Response[Error | ListTokenTransfersV2Response200]:
    """List SEP-41 token transfers (v2)

     v2 passthrough — identical shape to v1.

    Args:
        id (str):
        network (ListTokenTransfersV2Network | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | ListTokenTransfersV2Response200]
    """

    kwargs = _get_kwargs(
        id=id,
        network=network,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    id: str,
    *,
    client: AuthenticatedClient,
    network: ListTokenTransfersV2Network | Unset = UNSET,
) -> Error | ListTokenTransfersV2Response200 | None:
    """List SEP-41 token transfers (v2)

     v2 passthrough — identical shape to v1.

    Args:
        id (str):
        network (ListTokenTransfersV2Network | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | ListTokenTransfersV2Response200
    """

    return sync_detailed(
        id=id,
        client=client,
        network=network,
    ).parsed


async def asyncio_detailed(
    id: str,
    *,
    client: AuthenticatedClient,
    network: ListTokenTransfersV2Network | Unset = UNSET,
) -> Response[Error | ListTokenTransfersV2Response200]:
    """List SEP-41 token transfers (v2)

     v2 passthrough — identical shape to v1.

    Args:
        id (str):
        network (ListTokenTransfersV2Network | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | ListTokenTransfersV2Response200]
    """

    kwargs = _get_kwargs(
        id=id,
        network=network,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    id: str,
    *,
    client: AuthenticatedClient,
    network: ListTokenTransfersV2Network | Unset = UNSET,
) -> Error | ListTokenTransfersV2Response200 | None:
    """List SEP-41 token transfers (v2)

     v2 passthrough — identical shape to v1.

    Args:
        id (str):
        network (ListTokenTransfersV2Network | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | ListTokenTransfersV2Response200
    """

    return (
        await asyncio_detailed(
            id=id,
            client=client,
            network=network,
        )
    ).parsed
