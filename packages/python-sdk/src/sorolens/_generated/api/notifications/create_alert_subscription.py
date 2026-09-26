from http import HTTPStatus
from typing import Any

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.alert_subscription import AlertSubscription
from ...models.create_alert_subscription import CreateAlertSubscription
from ...models.error import Error
from ...models.role_error import RoleError
from ...types import Response


def _get_kwargs(
    *,
    body: CreateAlertSubscription,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/api/v1/watchdog/subscriptions",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> AlertSubscription | Error | RoleError | None:
    if response.status_code == 201:
        response_201 = AlertSubscription.from_dict(response.json())

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
) -> Response[AlertSubscription | Error | RoleError]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient,
    body: CreateAlertSubscription,
) -> Response[AlertSubscription | Error | RoleError]:
    """Subscribe a channel to a contract's alerts

     Critical alerts for the contract are delivered to the channel in its
    native format: `webhook` (generic JSON), `slack` (Block Kit via an
    incoming webhook), `discord` (embed via a channel webhook) or
    `pagerduty` (Events API v2 trigger with severity mapping).

    Args:
        body (CreateAlertSubscription):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AlertSubscription | Error | RoleError]
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
    body: CreateAlertSubscription,
) -> AlertSubscription | Error | RoleError | None:
    """Subscribe a channel to a contract's alerts

     Critical alerts for the contract are delivered to the channel in its
    native format: `webhook` (generic JSON), `slack` (Block Kit via an
    incoming webhook), `discord` (embed via a channel webhook) or
    `pagerduty` (Events API v2 trigger with severity mapping).

    Args:
        body (CreateAlertSubscription):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AlertSubscription | Error | RoleError
    """

    return sync_detailed(
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient,
    body: CreateAlertSubscription,
) -> Response[AlertSubscription | Error | RoleError]:
    """Subscribe a channel to a contract's alerts

     Critical alerts for the contract are delivered to the channel in its
    native format: `webhook` (generic JSON), `slack` (Block Kit via an
    incoming webhook), `discord` (embed via a channel webhook) or
    `pagerduty` (Events API v2 trigger with severity mapping).

    Args:
        body (CreateAlertSubscription):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AlertSubscription | Error | RoleError]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient,
    body: CreateAlertSubscription,
) -> AlertSubscription | Error | RoleError | None:
    """Subscribe a channel to a contract's alerts

     Critical alerts for the contract are delivered to the channel in its
    native format: `webhook` (generic JSON), `slack` (Block Kit via an
    incoming webhook), `discord` (embed via a channel webhook) or
    `pagerduty` (Events API v2 trigger with severity mapping).

    Args:
        body (CreateAlertSubscription):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AlertSubscription | Error | RoleError
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
        )
    ).parsed
