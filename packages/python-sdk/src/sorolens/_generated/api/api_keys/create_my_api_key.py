from http import HTTPStatus
from typing import Any

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.create_my_api_key_body import CreateMyApiKeyBody
from ...models.create_my_api_key_response_201 import CreateMyApiKeyResponse201
from ...models.error import Error
from ...types import Response


def _get_kwargs(
    *,
    body: CreateMyApiKeyBody,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/api/v1/me/api-keys",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> CreateMyApiKeyResponse201 | Error | None:
    if response.status_code == 201:
        response_201 = CreateMyApiKeyResponse201.from_dict(response.json())

        return response_201

    if response.status_code == 401:
        response_401 = Error.from_dict(response.json())

        return response_401

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
) -> Response[CreateMyApiKeyResponse201 | Error]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient,
    body: CreateMyApiKeyBody,
) -> Response[CreateMyApiKeyResponse201 | Error]:
    """Create an API key for the signed-in user

     Creates a key bound to the workspace session's user. The plaintext key
    is returned once. `admin:*` cannot be granted here.

    Args:
        body (CreateMyApiKeyBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[CreateMyApiKeyResponse201 | Error]
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
    body: CreateMyApiKeyBody,
) -> CreateMyApiKeyResponse201 | Error | None:
    """Create an API key for the signed-in user

     Creates a key bound to the workspace session's user. The plaintext key
    is returned once. `admin:*` cannot be granted here.

    Args:
        body (CreateMyApiKeyBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        CreateMyApiKeyResponse201 | Error
    """

    return sync_detailed(
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient,
    body: CreateMyApiKeyBody,
) -> Response[CreateMyApiKeyResponse201 | Error]:
    """Create an API key for the signed-in user

     Creates a key bound to the workspace session's user. The plaintext key
    is returned once. `admin:*` cannot be granted here.

    Args:
        body (CreateMyApiKeyBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[CreateMyApiKeyResponse201 | Error]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient,
    body: CreateMyApiKeyBody,
) -> CreateMyApiKeyResponse201 | Error | None:
    """Create an API key for the signed-in user

     Creates a key bound to the workspace session's user. The plaintext key
    is returned once. `admin:*` cannot be granted here.

    Args:
        body (CreateMyApiKeyBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        CreateMyApiKeyResponse201 | Error
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
        )
    ).parsed
