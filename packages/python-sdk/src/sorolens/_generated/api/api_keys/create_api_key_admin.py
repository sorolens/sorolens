from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ...client import AuthenticatedClient, Client
from ...types import Response, UNSET
from ... import errors

from ...models.create_api_key_admin_body import CreateApiKeyAdminBody
from ...models.create_api_key_admin_response_201 import CreateApiKeyAdminResponse201
from ...models.error import Error
from ...models.role_error import RoleError
from typing import cast



def _get_kwargs(
    *,
    body: CreateApiKeyAdminBody,

) -> dict[str, Any]:
    headers: dict[str, Any] = {}


    

    

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/api/v1/admin/keys",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs



def _parse_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> CreateApiKeyAdminResponse201 | Error | RoleError | None:
    if response.status_code == 201:
        response_201 = CreateApiKeyAdminResponse201.from_dict(response.json())



        return response_201

    if response.status_code == 401:
        response_401 = Error.from_dict(response.json())



        return response_401

    if response.status_code == 403:
        response_403 = RoleError.from_dict(response.json())



        return response_403

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


def _build_response(*, client: AuthenticatedClient | Client, response: httpx.Response) -> Response[CreateApiKeyAdminResponse201 | Error | RoleError]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient,
    body: CreateApiKeyAdminBody,

) -> Response[CreateApiKeyAdminResponse201 | Error | RoleError]:
    """ Create an API key (admin role only)

    Args:
        body (CreateApiKeyAdminBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[CreateApiKeyAdminResponse201 | Error | RoleError]
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
    body: CreateApiKeyAdminBody,

) -> CreateApiKeyAdminResponse201 | Error | RoleError | None:
    """ Create an API key (admin role only)

    Args:
        body (CreateApiKeyAdminBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        CreateApiKeyAdminResponse201 | Error | RoleError
     """


    return sync_detailed(
        client=client,
body=body,

    ).parsed

async def asyncio_detailed(
    *,
    client: AuthenticatedClient,
    body: CreateApiKeyAdminBody,

) -> Response[CreateApiKeyAdminResponse201 | Error | RoleError]:
    """ Create an API key (admin role only)

    Args:
        body (CreateApiKeyAdminBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[CreateApiKeyAdminResponse201 | Error | RoleError]
     """


    kwargs = _get_kwargs(
        body=body,

    )

    response = await client.get_async_httpx_client().request(
        **kwargs
    )

    return _build_response(client=client, response=response)

async def asyncio(
    *,
    client: AuthenticatedClient,
    body: CreateApiKeyAdminBody,

) -> CreateApiKeyAdminResponse201 | Error | RoleError | None:
    """ Create an API key (admin role only)

    Args:
        body (CreateApiKeyAdminBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        CreateApiKeyAdminResponse201 | Error | RoleError
     """


    return (await asyncio_detailed(
        client=client,
body=body,

    )).parsed
