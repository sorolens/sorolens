from http import HTTPStatus
from typing import Any

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.add_watched_account_body import AddWatchedAccountBody
from ...models.error import Error
from ...models.role_error import RoleError
from ...models.watched_account import WatchedAccount
from ...types import Response


def _get_kwargs(
    *,
    body: AddWatchedAccountBody,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/api/v1/watched-accounts",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | RoleError | WatchedAccount | None:
    if response.status_code == 200:
        response_200 = WatchedAccount.from_dict(response.json())

        return response_200

    if response.status_code == 201:
        response_201 = WatchedAccount.from_dict(response.json())

        return response_201

    if response.status_code == 401:
        response_401 = Error.from_dict(response.json())

        return response_401

    if response.status_code == 403:
        response_403 = RoleError.from_dict(response.json())

        return response_403

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
) -> Response[Error | RoleError | WatchedAccount]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient,
    body: AddWatchedAccountBody,
) -> Response[Error | RoleError | WatchedAccount]:
    """Watch an account for new contract deployments

     Idempotent: returns 201 when the account is newly watched and 200 when it already was. Same role
    rules as contract registration.

    Args:
        body (AddWatchedAccountBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | RoleError | WatchedAccount]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient,
    body: AddWatchedAccountBody,
) -> Error | RoleError | WatchedAccount | None:
    """Watch an account for new contract deployments

     Idempotent: returns 201 when the account is newly watched and 200 when it already was. Same role
    rules as contract registration.

    Args:
        body (AddWatchedAccountBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | RoleError | WatchedAccount
    """

    return sync_detailed(
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient,
    body: AddWatchedAccountBody,
) -> Response[Error | RoleError | WatchedAccount]:
    """Watch an account for new contract deployments

     Idempotent: returns 201 when the account is newly watched and 200 when it already was. Same role
    rules as contract registration.

    Args:
        body (AddWatchedAccountBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | RoleError | WatchedAccount]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient,
    body: AddWatchedAccountBody,
) -> Error | RoleError | WatchedAccount | None:
    """Watch an account for new contract deployments

     Idempotent: returns 201 when the account is newly watched and 200 when it already was. Same role
    rules as contract registration.

    Args:
        body (AddWatchedAccountBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | RoleError | WatchedAccount
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
        )
    ).parsed
