from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.contract_snapshot_export import ContractSnapshotExport
from ...models.error import Error
from ...types import Response


def _get_kwargs(
    id: str,
) -> dict[str, Any]:

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v1/contracts/{id}/snapshot.json".format(
            id=quote(str(id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ContractSnapshotExport | Error | None:
    if response.status_code == 200:
        response_200 = ContractSnapshotExport.from_dict(response.json())

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
) -> Response[ContractSnapshotExport | Error]:
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
) -> Response[ContractSnapshotExport | Error]:
    """Portable JSON export of a contract's current state

     Returns the contract's current state as a single versioned JSON
    document: metadata, storage, recent events, and summary. The shape is
    frozen by `schema_version` and is deterministic for an unchanged store
    (stable field order, sorted collections, no wall-clock fields), so
    exports can be archived or diffed. When the client advertises
    `Accept-Encoding: gzip` the body is gzip-compressed.

    Args:
        id (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContractSnapshotExport | Error]
    """

    kwargs = _get_kwargs(
        id=id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    id: str,
    *,
    client: AuthenticatedClient,
) -> ContractSnapshotExport | Error | None:
    """Portable JSON export of a contract's current state

     Returns the contract's current state as a single versioned JSON
    document: metadata, storage, recent events, and summary. The shape is
    frozen by `schema_version` and is deterministic for an unchanged store
    (stable field order, sorted collections, no wall-clock fields), so
    exports can be archived or diffed. When the client advertises
    `Accept-Encoding: gzip` the body is gzip-compressed.

    Args:
        id (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContractSnapshotExport | Error
    """

    return sync_detailed(
        id=id,
        client=client,
    ).parsed


async def asyncio_detailed(
    id: str,
    *,
    client: AuthenticatedClient,
) -> Response[ContractSnapshotExport | Error]:
    """Portable JSON export of a contract's current state

     Returns the contract's current state as a single versioned JSON
    document: metadata, storage, recent events, and summary. The shape is
    frozen by `schema_version` and is deterministic for an unchanged store
    (stable field order, sorted collections, no wall-clock fields), so
    exports can be archived or diffed. When the client advertises
    `Accept-Encoding: gzip` the body is gzip-compressed.

    Args:
        id (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContractSnapshotExport | Error]
    """

    kwargs = _get_kwargs(
        id=id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    id: str,
    *,
    client: AuthenticatedClient,
) -> ContractSnapshotExport | Error | None:
    """Portable JSON export of a contract's current state

     Returns the contract's current state as a single versioned JSON
    document: metadata, storage, recent events, and summary. The shape is
    frozen by `schema_version` and is deterministic for an unchanged store
    (stable field order, sorted collections, no wall-clock fields), so
    exports can be archived or diffed. When the client advertises
    `Accept-Encoding: gzip` the body is gzip-compressed.

    Args:
        id (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContractSnapshotExport | Error
    """

    return (
        await asyncio_detailed(
            id=id,
            client=client,
        )
    ).parsed
