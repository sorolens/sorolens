from http import HTTPStatus
from typing import Any

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.search_event_values_response_200 import SearchEventValuesResponse200
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    q: str,
    cursor: str | Unset = UNSET,
    limit: int | Unset = 50,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["q"] = q

    params["cursor"] = cursor

    params["limit"] = limit

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v1/events/search",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | SearchEventValuesResponse200 | None:
    if response.status_code == 200:
        response_200 = SearchEventValuesResponse200.from_dict(response.json())

        return response_200

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
) -> Response[Error | SearchEventValuesResponse200]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient,
    q: str,
    cursor: str | Unset = UNSET,
    limit: int | Unset = 50,
) -> Response[Error | SearchEventValuesResponse200]:
    """Full-text search over decoded event values

     Returns events whose decoded topic or value JSONB matches the query
    using the event_tsv full-text index (issue #798). Results are newest-
    first and cursor-paginated.

    Args:
        q (str):
        cursor (str | Unset):
        limit (int | Unset):  Default: 50.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | SearchEventValuesResponse200]
    """

    kwargs = _get_kwargs(
        q=q,
        cursor=cursor,
        limit=limit,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient,
    q: str,
    cursor: str | Unset = UNSET,
    limit: int | Unset = 50,
) -> Error | SearchEventValuesResponse200 | None:
    """Full-text search over decoded event values

     Returns events whose decoded topic or value JSONB matches the query
    using the event_tsv full-text index (issue #798). Results are newest-
    first and cursor-paginated.

    Args:
        q (str):
        cursor (str | Unset):
        limit (int | Unset):  Default: 50.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | SearchEventValuesResponse200
    """

    return sync_detailed(
        client=client,
        q=q,
        cursor=cursor,
        limit=limit,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient,
    q: str,
    cursor: str | Unset = UNSET,
    limit: int | Unset = 50,
) -> Response[Error | SearchEventValuesResponse200]:
    """Full-text search over decoded event values

     Returns events whose decoded topic or value JSONB matches the query
    using the event_tsv full-text index (issue #798). Results are newest-
    first and cursor-paginated.

    Args:
        q (str):
        cursor (str | Unset):
        limit (int | Unset):  Default: 50.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | SearchEventValuesResponse200]
    """

    kwargs = _get_kwargs(
        q=q,
        cursor=cursor,
        limit=limit,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient,
    q: str,
    cursor: str | Unset = UNSET,
    limit: int | Unset = 50,
) -> Error | SearchEventValuesResponse200 | None:
    """Full-text search over decoded event values

     Returns events whose decoded topic or value JSONB matches the query
    using the event_tsv full-text index (issue #798). Results are newest-
    first and cursor-paginated.

    Args:
        q (str):
        cursor (str | Unset):
        limit (int | Unset):  Default: 50.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | SearchEventValuesResponse200
    """

    return (
        await asyncio_detailed(
            client=client,
            q=q,
            cursor=cursor,
            limit=limit,
        )
    ).parsed
