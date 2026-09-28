from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.group import Group
from ...models.update_group_body import UpdateGroupBody
from ...types import Response


def _get_kwargs(
    id: UUID,
    *,
    body: UpdateGroupBody,
    x_user_id: str,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    headers["X-User-ID"] = x_user_id

    _kwargs: dict[str, Any] = {
        "method": "patch",
        "url": "/api/v1/groups/{id}".format(
            id=quote(str(id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | Group | None:
    if response.status_code == 200:
        response_200 = Group.from_dict(response.json())

        return response_200

    if response.status_code == 401:
        response_401 = Error.from_dict(response.json())

        return response_401

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
) -> Response[Error | Group]:
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
    body: UpdateGroupBody,
    x_user_id: str,
) -> Response[Error | Group]:
    """Rename a group

    Args:
        id (UUID):
        x_user_id (str):
        body (UpdateGroupBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | Group]
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
    body: UpdateGroupBody,
    x_user_id: str,
) -> Error | Group | None:
    """Rename a group

    Args:
        id (UUID):
        x_user_id (str):
        body (UpdateGroupBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | Group
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
    body: UpdateGroupBody,
    x_user_id: str,
) -> Response[Error | Group]:
    """Rename a group

    Args:
        id (UUID):
        x_user_id (str):
        body (UpdateGroupBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | Group]
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
    body: UpdateGroupBody,
    x_user_id: str,
) -> Error | Group | None:
    """Rename a group

    Args:
        id (UUID):
        x_user_id (str):
        body (UpdateGroupBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | Group
    """

    return (
        await asyncio_detailed(
            id=id,
            client=client,
            body=body,
            x_user_id=x_user_id,
        )
    ).parsed
