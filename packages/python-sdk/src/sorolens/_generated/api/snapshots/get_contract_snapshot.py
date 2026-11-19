from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.contract_snapshot import ContractSnapshot
from ...models.error import Error
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
        "url": "/api/v1/contracts/{id}/snapshot".format(
            id=quote(str(id), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ContractSnapshot | Error | None:
    if response.status_code == 200:
        response_200 = ContractSnapshot.from_dict(response.json())

        return response_200

    if response.status_code == 404:
        response_404 = Error.from_dict(response.json())

        return response_404

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
) -> Response[ContractSnapshot | Error]:
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
) -> Response[ContractSnapshot | Error]:
    """Snapshot of contract storage at a historical ledger

    Args:
        id (str):
        ledger (int):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContractSnapshot | Error]
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
) -> ContractSnapshot | Error | None:
    """Snapshot of contract storage at a historical ledger

    Args:
        id (str):
        ledger (int):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContractSnapshot | Error
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
) -> Response[ContractSnapshot | Error]:
    """Snapshot of contract storage at a historical ledger

    Args:
        id (str):
        ledger (int):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContractSnapshot | Error]
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
) -> ContractSnapshot | Error | None:
    """Snapshot of contract storage at a historical ledger

    Args:
        id (str):
        ledger (int):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContractSnapshot | Error
    """

    return (
        await asyncio_detailed(
            id=id,
            client=client,
            ledger=ledger,
        )
    ).parsed
