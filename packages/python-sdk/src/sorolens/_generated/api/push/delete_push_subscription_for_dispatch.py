from http import HTTPStatus
from typing import Any, cast

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.delete_push_subscription_for_dispatch_body import (
    DeletePushSubscriptionForDispatchBody,
)
from ...models.error import Error
from ...types import Response


def _get_kwargs(
    *,
    body: DeletePushSubscriptionForDispatchBody,
    x_push_secret: str,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    headers["X-Push-Secret"] = x_push_secret

    _kwargs: dict[str, Any] = {
        "method": "delete",
        "url": "/api/v1/push/subscriptions/dispatch",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Any | Error | None:
    if response.status_code == 204:
        response_204 = cast(Any, None)
        return response_204

    if response.status_code == 401:
        response_401 = Error.from_dict(response.json())

        return response_401

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Any | Error]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: DeletePushSubscriptionForDispatchBody,
    x_push_secret: str,
) -> Response[Any | Error]:
    """Prune an expired subscription reported by a push service

     Internal route guarded by the `X-Push-Secret` header.

    Args:
        x_push_secret (str):
        body (DeletePushSubscriptionForDispatchBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Error]
    """

    kwargs = _get_kwargs(
        body=body,
        x_push_secret=x_push_secret,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    body: DeletePushSubscriptionForDispatchBody,
    x_push_secret: str,
) -> Any | Error | None:
    """Prune an expired subscription reported by a push service

     Internal route guarded by the `X-Push-Secret` header.

    Args:
        x_push_secret (str):
        body (DeletePushSubscriptionForDispatchBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Error
    """

    return sync_detailed(
        client=client,
        body=body,
        x_push_secret=x_push_secret,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: DeletePushSubscriptionForDispatchBody,
    x_push_secret: str,
) -> Response[Any | Error]:
    """Prune an expired subscription reported by a push service

     Internal route guarded by the `X-Push-Secret` header.

    Args:
        x_push_secret (str):
        body (DeletePushSubscriptionForDispatchBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Error]
    """

    kwargs = _get_kwargs(
        body=body,
        x_push_secret=x_push_secret,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: DeletePushSubscriptionForDispatchBody,
    x_push_secret: str,
) -> Any | Error | None:
    """Prune an expired subscription reported by a push service

     Internal route guarded by the `X-Push-Secret` header.

    Args:
        x_push_secret (str):
        body (DeletePushSubscriptionForDispatchBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Error
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
            x_push_secret=x_push_secret,
        )
    ).parsed
