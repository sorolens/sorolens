from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.contract_note import ContractNote
from ...models.contract_note_request import ContractNoteRequest
from ...models.error import Error
from ...models.role_error import RoleError
from ...types import Response


def _get_kwargs(
    id: str,
    *,
    body: ContractNoteRequest,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/api/v1/contracts/{id}/notes".format(
            id=quote(str(id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ContractNote | Error | RoleError | None:
    if response.status_code == 201:
        response_201 = ContractNote.from_dict(response.json())

        return response_201

    if response.status_code == 401:
        response_401 = Error.from_dict(response.json())

        return response_401

    if response.status_code == 403:
        response_403 = RoleError.from_dict(response.json())

        return response_403

    if response.status_code == 404:
        response_404 = Error.from_dict(response.json())

        return response_404

    if response.status_code == 415:
        response_415 = Error.from_dict(response.json())

        return response_415

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
) -> Response[ContractNote | Error | RoleError]:
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
    body: ContractNoteRequest,
) -> Response[ContractNote | Error | RoleError]:
    """Add a note to a contract

     Appends a markdown note to a contract. The stored author is the caller
    identity (X-User-ID), which is what later authorizes editing and
    deleting the note. Authoring requires the contributor role.

    Args:
        id (str):
        body (ContractNoteRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContractNote | Error | RoleError]
    """

    kwargs = _get_kwargs(
        id=id,
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    id: str,
    *,
    client: AuthenticatedClient,
    body: ContractNoteRequest,
) -> ContractNote | Error | RoleError | None:
    """Add a note to a contract

     Appends a markdown note to a contract. The stored author is the caller
    identity (X-User-ID), which is what later authorizes editing and
    deleting the note. Authoring requires the contributor role.

    Args:
        id (str):
        body (ContractNoteRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContractNote | Error | RoleError
    """

    return sync_detailed(
        id=id,
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    id: str,
    *,
    client: AuthenticatedClient,
    body: ContractNoteRequest,
) -> Response[ContractNote | Error | RoleError]:
    """Add a note to a contract

     Appends a markdown note to a contract. The stored author is the caller
    identity (X-User-ID), which is what later authorizes editing and
    deleting the note. Authoring requires the contributor role.

    Args:
        id (str):
        body (ContractNoteRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContractNote | Error | RoleError]
    """

    kwargs = _get_kwargs(
        id=id,
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    id: str,
    *,
    client: AuthenticatedClient,
    body: ContractNoteRequest,
) -> ContractNote | Error | RoleError | None:
    """Add a note to a contract

     Appends a markdown note to a contract. The stored author is the caller
    identity (X-User-ID), which is what later authorizes editing and
    deleting the note. Authoring requires the contributor role.

    Args:
        id (str):
        body (ContractNoteRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContractNote | Error | RoleError
    """

    return (
        await asyncio_detailed(
            id=id,
            client=client,
            body=body,
        )
    ).parsed
