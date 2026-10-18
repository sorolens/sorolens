from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.invocation_detail import InvocationDetail
from ...types import Response


def _get_kwargs(
    id: str,
    tx_hash: str,
) -> dict[str, Any]:

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v1/contracts/{id}/invocations/{tx_hash}".format(
            id=quote(str(id), safe=""),
            tx_hash=quote(str(tx_hash), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | InvocationDetail | None:
    if response.status_code == 200:
        response_200 = InvocationDetail.from_dict(response.json())

        return response_200

    if response.status_code == 404:
        response_404 = Error.from_dict(response.json())

        return response_404

    if response.status_code == 500:
        response_500 = Error.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Error | InvocationDetail]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    id: str,
    tx_hash: str,
    *,
    client: AuthenticatedClient,
) -> Response[Error | InvocationDetail]:
    """Get contract invocation details with associated events

    Args:
        id (str):
        tx_hash (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | InvocationDetail]
    """

    kwargs = _get_kwargs(
        id=id,
        tx_hash=tx_hash,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    id: str,
    tx_hash: str,
    *,
    client: AuthenticatedClient,
) -> Error | InvocationDetail | None:
    """Get contract invocation details with associated events

    Args:
        id (str):
        tx_hash (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | InvocationDetail
    """

    return sync_detailed(
        id=id,
        tx_hash=tx_hash,
        client=client,
    ).parsed


async def asyncio_detailed(
    id: str,
    tx_hash: str,
    *,
    client: AuthenticatedClient,
) -> Response[Error | InvocationDetail]:
    """Get contract invocation details with associated events

    Args:
        id (str):
        tx_hash (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | InvocationDetail]
    """

    kwargs = _get_kwargs(
        id=id,
        tx_hash=tx_hash,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    id: str,
    tx_hash: str,
    *,
    client: AuthenticatedClient,
) -> Error | InvocationDetail | None:
    """Get contract invocation details with associated events

    Args:
        id (str):
        tx_hash (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | InvocationDetail
    """

    return (
        await asyncio_detailed(
            id=id,
            tx_hash=tx_hash,
            client=client,
        )
    ).parsed
