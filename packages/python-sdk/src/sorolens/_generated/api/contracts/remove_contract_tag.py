from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.role_error import RoleError
from ...types import Response


def _get_kwargs(
    id: str,
    tag: str,
) -> dict[str, Any]:

    _kwargs: dict[str, Any] = {
        "method": "delete",
        "url": "/api/v1/contracts/{id}/tags/{tag}".format(
            id=quote(str(id), safe=""),
            tag=quote(str(tag), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Any | Error | RoleError | None:
    if response.status_code == 204:
        response_204 = cast(Any, None)
        return response_204

    if response.status_code == 401:
        response_401 = Error.from_dict(response.json())

        return response_401

    if response.status_code == 403:
        response_403 = RoleError.from_dict(response.json())

        return response_403

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
) -> Response[Any | Error | RoleError]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    id: str,
    tag: str,
    *,
    client: AuthenticatedClient,
) -> Response[Any | Error | RoleError]:
    """Remove a tag from a contract

     Removes a user-defined tag from a contract. Removing a tag that is not
    present is a no-op.

    Args:
        id (str):
        tag (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Error | RoleError]
    """

    kwargs = _get_kwargs(
        id=id,
        tag=tag,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    id: str,
    tag: str,
    *,
    client: AuthenticatedClient,
) -> Any | Error | RoleError | None:
    """Remove a tag from a contract

     Removes a user-defined tag from a contract. Removing a tag that is not
    present is a no-op.

    Args:
        id (str):
        tag (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Error | RoleError
    """

    return sync_detailed(
        id=id,
        tag=tag,
        client=client,
    ).parsed


async def asyncio_detailed(
    id: str,
    tag: str,
    *,
    client: AuthenticatedClient,
) -> Response[Any | Error | RoleError]:
    """Remove a tag from a contract

     Removes a user-defined tag from a contract. Removing a tag that is not
    present is a no-op.

    Args:
        id (str):
        tag (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Error | RoleError]
    """

    kwargs = _get_kwargs(
        id=id,
        tag=tag,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    id: str,
    tag: str,
    *,
    client: AuthenticatedClient,
) -> Any | Error | RoleError | None:
    """Remove a tag from a contract

     Removes a user-defined tag from a contract. Removing a tag that is not
    present is a no-op.

    Args:
        id (str):
        tag (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Error | RoleError
    """

    return (
        await asyncio_detailed(
            id=id,
            tag=tag,
            client=client,
        )
    ).parsed
