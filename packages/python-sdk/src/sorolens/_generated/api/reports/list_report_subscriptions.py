from http import HTTPStatus
from typing import Any

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.list_report_subscriptions_response_200 import (
    ListReportSubscriptionsResponse200,
)
from ...types import UNSET, Response


def _get_kwargs(
    *,
    email: str,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["email"] = email

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v1/reports/subscriptions",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | ListReportSubscriptionsResponse200 | None:
    if response.status_code == 200:
        response_200 = ListReportSubscriptionsResponse200.from_dict(response.json())

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
) -> Response[Error | ListReportSubscriptionsResponse200]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    email: str,
) -> Response[Error | ListReportSubscriptionsResponse200]:
    """List a subscriber's digest subscriptions

    Args:
        email (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | ListReportSubscriptionsResponse200]
    """

    kwargs = _get_kwargs(
        email=email,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    email: str,
) -> Error | ListReportSubscriptionsResponse200 | None:
    """List a subscriber's digest subscriptions

    Args:
        email (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | ListReportSubscriptionsResponse200
    """

    return sync_detailed(
        client=client,
        email=email,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    email: str,
) -> Response[Error | ListReportSubscriptionsResponse200]:
    """List a subscriber's digest subscriptions

    Args:
        email (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | ListReportSubscriptionsResponse200]
    """

    kwargs = _get_kwargs(
        email=email,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    email: str,
) -> Error | ListReportSubscriptionsResponse200 | None:
    """List a subscriber's digest subscriptions

    Args:
        email (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | ListReportSubscriptionsResponse200
    """

    return (
        await asyncio_detailed(
            client=client,
            email=email,
        )
    ).parsed
