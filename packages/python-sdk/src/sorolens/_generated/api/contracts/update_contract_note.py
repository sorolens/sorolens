from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

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
    note_id: UUID,
    *,
    body: ContractNoteRequest,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "patch",
        "url": "/api/v1/contracts/{id}/notes/{note_id}".format(
            id=quote(str(id), safe=""),
            note_id=quote(str(note_id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ContractNote | Error | RoleError | None:
    if response.status_code == 200:
        response_200 = ContractNote.from_dict(response.json())

        return response_200

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
    note_id: UUID,
    *,
    client: AuthenticatedClient,
    body: ContractNoteRequest,
) -> Response[ContractNote | Error | RoleError]:
    """Rewrite a note

     Replaces a note's body. Only the note's author may rewrite it; any
    other contributor gets 403. The stored author is never reassigned.

    Args:
        id (str):
        note_id (UUID):
        body (ContractNoteRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContractNote | Error | RoleError]
    """

    kwargs = _get_kwargs(
        id=id,
        note_id=note_id,
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    id: str,
    note_id: UUID,
    *,
    client: AuthenticatedClient,
    body: ContractNoteRequest,
) -> ContractNote | Error | RoleError | None:
    """Rewrite a note

     Replaces a note's body. Only the note's author may rewrite it; any
    other contributor gets 403. The stored author is never reassigned.

    Args:
        id (str):
        note_id (UUID):
        body (ContractNoteRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContractNote | Error | RoleError
    """

    return sync_detailed(
        id=id,
        note_id=note_id,
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    id: str,
    note_id: UUID,
    *,
    client: AuthenticatedClient,
    body: ContractNoteRequest,
) -> Response[ContractNote | Error | RoleError]:
    """Rewrite a note

     Replaces a note's body. Only the note's author may rewrite it; any
    other contributor gets 403. The stored author is never reassigned.

    Args:
        id (str):
        note_id (UUID):
        body (ContractNoteRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContractNote | Error | RoleError]
    """

    kwargs = _get_kwargs(
        id=id,
        note_id=note_id,
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    id: str,
    note_id: UUID,
    *,
    client: AuthenticatedClient,
    body: ContractNoteRequest,
) -> ContractNote | Error | RoleError | None:
    """Rewrite a note

     Replaces a note's body. Only the note's author may rewrite it; any
    other contributor gets 403. The stored author is never reassigned.

    Args:
        id (str):
        note_id (UUID):
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
            note_id=note_id,
            client=client,
            body=body,
        )
    ).parsed
