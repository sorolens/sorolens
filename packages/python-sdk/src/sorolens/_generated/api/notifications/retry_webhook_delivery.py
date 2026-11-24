from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.role_error import RoleError
from ...models.webhook_delivery import WebhookDelivery
from ...types import Response


def _get_kwargs(
    id: str,
    delivery_id: str,
) -> dict[str, Any]:

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/api/v1/watchdog/subscriptions/{id}/deliveries/{delivery_id}/retry".format(
            id=quote(str(id), safe=""),
            delivery_id=quote(str(delivery_id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | RoleError | WebhookDelivery | None:
    if response.status_code == 202:
        response_202 = WebhookDelivery.from_dict(response.json())

        return response_202

    if response.status_code == 401:
        response_401 = Error.from_dict(response.json())

        return response_401

    if response.status_code == 403:
        response_403 = RoleError.from_dict(response.json())

        return response_403

    if response.status_code == 404:
        response_404 = Error.from_dict(response.json())

        return response_404

    if response.status_code == 500:
        response_500 = Error.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Error | RoleError | WebhookDelivery]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    id: str,
    delivery_id: str,
    *,
    client: AuthenticatedClient,
) -> Response[Error | RoleError | WebhookDelivery]:
    """Manually retry a failed webhook delivery

     Marks a failed delivery for immediate retry by resetting its
    next_retry_at to now and incrementing the attempt counter.
    Only contributor+ callers may trigger a retry (issue #828).

    Args:
        id (str):
        delivery_id (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | RoleError | WebhookDelivery]
    """

    kwargs = _get_kwargs(
        id=id,
        delivery_id=delivery_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    id: str,
    delivery_id: str,
    *,
    client: AuthenticatedClient,
) -> Error | RoleError | WebhookDelivery | None:
    """Manually retry a failed webhook delivery

     Marks a failed delivery for immediate retry by resetting its
    next_retry_at to now and incrementing the attempt counter.
    Only contributor+ callers may trigger a retry (issue #828).

    Args:
        id (str):
        delivery_id (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | RoleError | WebhookDelivery
    """

    return sync_detailed(
        id=id,
        delivery_id=delivery_id,
        client=client,
    ).parsed


async def asyncio_detailed(
    id: str,
    delivery_id: str,
    *,
    client: AuthenticatedClient,
) -> Response[Error | RoleError | WebhookDelivery]:
    """Manually retry a failed webhook delivery

     Marks a failed delivery for immediate retry by resetting its
    next_retry_at to now and incrementing the attempt counter.
    Only contributor+ callers may trigger a retry (issue #828).

    Args:
        id (str):
        delivery_id (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | RoleError | WebhookDelivery]
    """

    kwargs = _get_kwargs(
        id=id,
        delivery_id=delivery_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    id: str,
    delivery_id: str,
    *,
    client: AuthenticatedClient,
) -> Error | RoleError | WebhookDelivery | None:
    """Manually retry a failed webhook delivery

     Marks a failed delivery for immediate retry by resetting its
    next_retry_at to now and incrementing the attempt counter.
    Only contributor+ callers may trigger a retry (issue #828).

    Args:
        id (str):
        delivery_id (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | RoleError | WebhookDelivery
    """

    return (
        await asyncio_detailed(
            id=id,
            delivery_id=delivery_id,
            client=client,
        )
    ).parsed
