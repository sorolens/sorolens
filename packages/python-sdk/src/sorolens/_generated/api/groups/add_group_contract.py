from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.add_group_contract_body import AddGroupContractBody
from ...models.error import Error
from ...models.group_membership import GroupMembership
from ...types import Response


def _get_kwargs(
    id: UUID,
    *,
    body: AddGroupContractBody,
    x_user_id: str,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    headers["X-User-ID"] = x_user_id

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/api/v1/groups/{id}/contracts".format(
            id=quote(str(id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | GroupMembership | None:
    if response.status_code == 201:
        response_201 = GroupMembership.from_dict(response.json())

        return response_201

    if response.status_code == 401:
        response_401 = Error.from_dict(response.json())

        return response_401

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
) -> Response[Error | GroupMembership]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    id: UUID,
    *,
    client: AuthenticatedClient,
    body: AddGroupContractBody,
    x_user_id: str,
) -> Response[Error | GroupMembership]:
    """Add a contract to a group

     Membership is many-to-many and idempotent: a contract may belong to any
    number of groups, and re-adding an existing member succeeds.

    Args:
        id (UUID):
        x_user_id (str):
        body (AddGroupContractBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | GroupMembership]
    """

    kwargs = _get_kwargs(
        id=id,
        body=body,
        x_user_id=x_user_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    id: UUID,
    *,
    client: AuthenticatedClient,
    body: AddGroupContractBody,
    x_user_id: str,
) -> Error | GroupMembership | None:
    """Add a contract to a group

     Membership is many-to-many and idempotent: a contract may belong to any
    number of groups, and re-adding an existing member succeeds.

    Args:
        id (UUID):
        x_user_id (str):
        body (AddGroupContractBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | GroupMembership
    """

    return sync_detailed(
        id=id,
        client=client,
        body=body,
        x_user_id=x_user_id,
    ).parsed


async def asyncio_detailed(
    id: UUID,
    *,
    client: AuthenticatedClient,
    body: AddGroupContractBody,
    x_user_id: str,
) -> Response[Error | GroupMembership]:
    """Add a contract to a group

     Membership is many-to-many and idempotent: a contract may belong to any
    number of groups, and re-adding an existing member succeeds.

    Args:
        id (UUID):
        x_user_id (str):
        body (AddGroupContractBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | GroupMembership]
    """

    kwargs = _get_kwargs(
        id=id,
        body=body,
        x_user_id=x_user_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    id: UUID,
    *,
    client: AuthenticatedClient,
    body: AddGroupContractBody,
    x_user_id: str,
) -> Error | GroupMembership | None:
    """Add a contract to a group

     Membership is many-to-many and idempotent: a contract may belong to any
    number of groups, and re-adding an existing member succeeds.

    Args:
        id (UUID):
        x_user_id (str):
        body (AddGroupContractBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | GroupMembership
    """

    return (
        await asyncio_detailed(
            id=id,
            client=client,
            body=body,
            x_user_id=x_user_id,
        )
    ).parsed
