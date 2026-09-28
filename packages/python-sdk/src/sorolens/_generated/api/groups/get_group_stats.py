from http import HTTPStatus
from typing import Any
from urllib.parse import quote
from uuid import UUID

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.group_stats import GroupStats
from ...types import Response


def _get_kwargs(
    id: UUID,
    *,
    x_user_id: str,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    headers["X-User-ID"] = x_user_id

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v1/groups/{id}/stats".format(
            id=quote(str(id), safe=""),
        ),
    }

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | GroupStats | None:
    if response.status_code == 200:
        response_200 = GroupStats.from_dict(response.json())

        return response_200

    if response.status_code == 401:
        response_401 = Error.from_dict(response.json())

        return response_401

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
) -> Response[Error | GroupStats]:
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
    x_user_id: str,
) -> Response[Error | GroupStats]:
    """Aggregate statistics across a group's contracts

     Aggregates the event count, invocation count, storage entry count, and
    average cached health score across every contract in the group. The
    average is taken over members that have a cached health score.

    Args:
        id (UUID):
        x_user_id (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | GroupStats]
    """

    kwargs = _get_kwargs(
        id=id,
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
    x_user_id: str,
) -> Error | GroupStats | None:
    """Aggregate statistics across a group's contracts

     Aggregates the event count, invocation count, storage entry count, and
    average cached health score across every contract in the group. The
    average is taken over members that have a cached health score.

    Args:
        id (UUID):
        x_user_id (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | GroupStats
    """

    return sync_detailed(
        id=id,
        client=client,
        x_user_id=x_user_id,
    ).parsed


async def asyncio_detailed(
    id: UUID,
    *,
    client: AuthenticatedClient,
    x_user_id: str,
) -> Response[Error | GroupStats]:
    """Aggregate statistics across a group's contracts

     Aggregates the event count, invocation count, storage entry count, and
    average cached health score across every contract in the group. The
    average is taken over members that have a cached health score.

    Args:
        id (UUID):
        x_user_id (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | GroupStats]
    """

    kwargs = _get_kwargs(
        id=id,
        x_user_id=x_user_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    id: UUID,
    *,
    client: AuthenticatedClient,
    x_user_id: str,
) -> Error | GroupStats | None:
    """Aggregate statistics across a group's contracts

     Aggregates the event count, invocation count, storage entry count, and
    average cached health score across every contract in the group. The
    average is taken over members that have a cached health score.

    Args:
        id (UUID):
        x_user_id (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | GroupStats
    """

    return (
        await asyncio_detailed(
            id=id,
            client=client,
            x_user_id=x_user_id,
        )
    ).parsed
