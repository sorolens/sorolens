from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.get_contract_resource_trend_v2_response_200 import (
    GetContractResourceTrendV2Response200,
)
from ...types import UNSET, Response, Unset


def _get_kwargs(
    id: str,
    *,
    days: int | Unset = 30,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["days"] = days

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v2/contracts/{id}/resource-trend".format(
            id=quote(str(id), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | GetContractResourceTrendV2Response200 | None:
    if response.status_code == 200:
        response_200 = GetContractResourceTrendV2Response200.from_dict(response.json())

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
) -> Response[Error | GetContractResourceTrendV2Response200]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    id: str,
    *,
    client: AuthenticatedClient,
    days: int | Unset = 30,
) -> Response[Error | GetContractResourceTrendV2Response200]:
    """Daily resource usage trend for a contract (v2)

     v2 alias of `GET /api/v1/contracts/{id}/resource-trend`. Aggregates invocations into UTC-day buckets
    in one request.

    Args:
        id (str):
        days (int | Unset):  Default: 30.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | GetContractResourceTrendV2Response200]
    """

    kwargs = _get_kwargs(
        id=id,
        days=days,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    id: str,
    *,
    client: AuthenticatedClient,
    days: int | Unset = 30,
) -> Error | GetContractResourceTrendV2Response200 | None:
    """Daily resource usage trend for a contract (v2)

     v2 alias of `GET /api/v1/contracts/{id}/resource-trend`. Aggregates invocations into UTC-day buckets
    in one request.

    Args:
        id (str):
        days (int | Unset):  Default: 30.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | GetContractResourceTrendV2Response200
    """

    return sync_detailed(
        id=id,
        client=client,
        days=days,
    ).parsed


async def asyncio_detailed(
    id: str,
    *,
    client: AuthenticatedClient,
    days: int | Unset = 30,
) -> Response[Error | GetContractResourceTrendV2Response200]:
    """Daily resource usage trend for a contract (v2)

     v2 alias of `GET /api/v1/contracts/{id}/resource-trend`. Aggregates invocations into UTC-day buckets
    in one request.

    Args:
        id (str):
        days (int | Unset):  Default: 30.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | GetContractResourceTrendV2Response200]
    """

    kwargs = _get_kwargs(
        id=id,
        days=days,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    id: str,
    *,
    client: AuthenticatedClient,
    days: int | Unset = 30,
) -> Error | GetContractResourceTrendV2Response200 | None:
    """Daily resource usage trend for a contract (v2)

     v2 alias of `GET /api/v1/contracts/{id}/resource-trend`. Aggregates invocations into UTC-day buckets
    in one request.

    Args:
        id (str):
        days (int | Unset):  Default: 30.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | GetContractResourceTrendV2Response200
    """

    return (
        await asyncio_detailed(
            id=id,
            client=client,
            days=days,
        )
    ).parsed
