from http import HTTPStatus
from typing import Any

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.graphql_body import GraphqlBody
from ...models.graphql_response_200 import GraphqlResponse200
from ...models.graphql_response_400 import GraphqlResponse400
from ...models.role_error import RoleError
from ...types import Response


def _get_kwargs(
    *,
    body: GraphqlBody,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/graphql",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | GraphqlResponse200 | GraphqlResponse400 | RoleError | None:
    if response.status_code == 200:
        response_200 = GraphqlResponse200.from_dict(response.json())

        return response_200

    if response.status_code == 400:
        response_400 = GraphqlResponse400.from_dict(response.json())

        return response_400

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

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Error | GraphqlResponse200 | GraphqlResponse400 | RoleError]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient,
    body: GraphqlBody,
) -> Response[Error | GraphqlResponse200 | GraphqlResponse400 | RoleError]:
    """Read-only GraphQL endpoint

     GraphQL query surface over contracts, events, invocations, storage, stats and watchdog data. POST
    only, GraphQL requests only. Anonymous requests are allowed; an API key needs read:contracts. Query
    complexity is capped (GRAPHQL_COMPLEXITY_LIMIT) and every list argument at 100.

    Args:
        body (GraphqlBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | GraphqlResponse200 | GraphqlResponse400 | RoleError]
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
    body: GraphqlBody,
) -> Error | GraphqlResponse200 | GraphqlResponse400 | RoleError | None:
    """Read-only GraphQL endpoint

     GraphQL query surface over contracts, events, invocations, storage, stats and watchdog data. POST
    only, GraphQL requests only. Anonymous requests are allowed; an API key needs read:contracts. Query
    complexity is capped (GRAPHQL_COMPLEXITY_LIMIT) and every list argument at 100.

    Args:
        body (GraphqlBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | GraphqlResponse200 | GraphqlResponse400 | RoleError
    """

    return sync_detailed(
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient,
    body: GraphqlBody,
) -> Response[Error | GraphqlResponse200 | GraphqlResponse400 | RoleError]:
    """Read-only GraphQL endpoint

     GraphQL query surface over contracts, events, invocations, storage, stats and watchdog data. POST
    only, GraphQL requests only. Anonymous requests are allowed; an API key needs read:contracts. Query
    complexity is capped (GRAPHQL_COMPLEXITY_LIMIT) and every list argument at 100.

    Args:
        body (GraphqlBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | GraphqlResponse200 | GraphqlResponse400 | RoleError]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient,
    body: GraphqlBody,
) -> Error | GraphqlResponse200 | GraphqlResponse400 | RoleError | None:
    """Read-only GraphQL endpoint

     GraphQL query surface over contracts, events, invocations, storage, stats and watchdog data. POST
    only, GraphQL requests only. Anonymous requests are allowed; an API key needs read:contracts. Query
    complexity is capped (GRAPHQL_COMPLEXITY_LIMIT) and every list argument at 100.

    Args:
        body (GraphqlBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | GraphqlResponse200 | GraphqlResponse400 | RoleError
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
        )
    ).parsed
