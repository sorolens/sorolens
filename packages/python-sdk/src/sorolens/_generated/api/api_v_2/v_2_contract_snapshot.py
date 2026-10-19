from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.v2_contract_snapshot_response_200 import V2ContractSnapshotResponse200
from ...types import UNSET, Response


def _get_kwargs(
    id: str,
    *,
    ledger: int,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["ledger"] = ledger

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v2/contracts/{id}/snapshot".format(
            id=quote(str(id), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | V2ContractSnapshotResponse200 | None:
    if response.status_code == 200:
        response_200 = V2ContractSnapshotResponse200.from_dict(response.json())

        return response_200

    if response.status_code == 404:
        response_404 = Error.from_dict(response.json())

        return response_404

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
) -> Response[Error | V2ContractSnapshotResponse200]:
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
    ledger: int,
) -> Response[Error | V2ContractSnapshotResponse200]:
    """Storage snapshot at a ledger (v1 shape)

    Args:
        id (str):
        ledger (int):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | V2ContractSnapshotResponse200]
    """

    kwargs = _get_kwargs(
        id=id,
        ledger=ledger,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    id: str,
    *,
    client: AuthenticatedClient,
    ledger: int,
) -> Error | V2ContractSnapshotResponse200 | None:
    """Storage snapshot at a ledger (v1 shape)

    Args:
        id (str):
        ledger (int):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | V2ContractSnapshotResponse200
    """

    return sync_detailed(
        id=id,
        client=client,
        ledger=ledger,
    ).parsed


async def asyncio_detailed(
    id: str,
    *,
    client: AuthenticatedClient,
    ledger: int,
) -> Response[Error | V2ContractSnapshotResponse200]:
    """Storage snapshot at a ledger (v1 shape)

    Args:
        id (str):
        ledger (int):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | V2ContractSnapshotResponse200]
    """

    kwargs = _get_kwargs(
        id=id,
        ledger=ledger,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    id: str,
    *,
    client: AuthenticatedClient,
    ledger: int,
) -> Error | V2ContractSnapshotResponse200 | None:
    """Storage snapshot at a ledger (v1 shape)

    Args:
        id (str):
        ledger (int):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | V2ContractSnapshotResponse200
    """

    return (
        await asyncio_detailed(
            id=id,
            client=client,
            ledger=ledger,
        )
    ).parsed
