from http import HTTPStatus
from typing import Any

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.session import Session
from ...types import Response


def _get_kwargs() -> dict[str, Any]:

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/api/v1/auth/session",
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | Session | None:
    if response.status_code == 201:
        response_201 = Session.from_dict(response.json())

        return response_201

    if response.status_code == 429:
        response_429 = Error.from_dict(response.json())

        return response_429

    if response.status_code == 503:
        response_503 = Error.from_dict(response.json())

        return response_503

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Error | Session]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
) -> Response[Error | Session]:
    """Mint a workspace session token

     Creates an unguessable workspace user with the contributor role and
    returns an HMAC-signed session token. Send it as
    `Authorization: Bearer <token>` on routes that need a verified user
    identity (watchlist, groups, notes, labels, write routes). The token is
    stateless: it remains valid until `expires_at`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | Session]
    """

    kwargs = _get_kwargs()

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
) -> Error | Session | None:
    """Mint a workspace session token

     Creates an unguessable workspace user with the contributor role and
    returns an HMAC-signed session token. Send it as
    `Authorization: Bearer <token>` on routes that need a verified user
    identity (watchlist, groups, notes, labels, write routes). The token is
    stateless: it remains valid until `expires_at`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | Session
    """

    return sync_detailed(
        client=client,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
) -> Response[Error | Session]:
    """Mint a workspace session token

     Creates an unguessable workspace user with the contributor role and
    returns an HMAC-signed session token. Send it as
    `Authorization: Bearer <token>` on routes that need a verified user
    identity (watchlist, groups, notes, labels, write routes). The token is
    stateless: it remains valid until `expires_at`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | Session]
    """

    kwargs = _get_kwargs()

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
) -> Error | Session | None:
    """Mint a workspace session token

     Creates an unguessable workspace user with the contributor role and
    returns an HMAC-signed session token. Send it as
    `Authorization: Bearer <token>` on routes that need a verified user
    identity (watchlist, groups, notes, labels, write routes). The token is
    stateless: it remains valid until `expires_at`.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | Session
    """

    return (
        await asyncio_detailed(
            client=client,
        )
    ).parsed
