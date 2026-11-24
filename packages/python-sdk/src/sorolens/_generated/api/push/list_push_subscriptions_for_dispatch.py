from http import HTTPStatus
from typing import Any, cast

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...types import Response


def _get_kwargs(
    *,
    x_push_secret: str,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    headers["X-Push-Secret"] = x_push_secret

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v1/push/subscriptions/dispatch",
    }

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Any | Error | None:
    if response.status_code == 200:
        response_200 = cast(Any, None)
        return response_200

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
    x_push_secret: str,
) -> Response[Any | Error]:
    """List every subscription for the internal push dispatcher

     Internal route for the alerting pipeline. Requires the
    `X-Push-Secret` header to match `PUSH_INTERNAL_SECRET`; it is not
    available to dashboard users.

    Args:
        x_push_secret (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Error]
    """

    kwargs = _get_kwargs(
        x_push_secret=x_push_secret,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    x_push_secret: str,
) -> Any | Error | None:
    """List every subscription for the internal push dispatcher

     Internal route for the alerting pipeline. Requires the
    `X-Push-Secret` header to match `PUSH_INTERNAL_SECRET`; it is not
    available to dashboard users.

    Args:
        x_push_secret (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Error
    """

    return sync_detailed(
        client=client,
        x_push_secret=x_push_secret,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    x_push_secret: str,
) -> Response[Any | Error]:
    """List every subscription for the internal push dispatcher

     Internal route for the alerting pipeline. Requires the
    `X-Push-Secret` header to match `PUSH_INTERNAL_SECRET`; it is not
    available to dashboard users.

    Args:
        x_push_secret (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Error]
    """

    kwargs = _get_kwargs(
        x_push_secret=x_push_secret,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    x_push_secret: str,
) -> Any | Error | None:
    """List every subscription for the internal push dispatcher

     Internal route for the alerting pipeline. Requires the
    `X-Push-Secret` header to match `PUSH_INTERNAL_SECRET`; it is not
    available to dashboard users.

    Args:
        x_push_secret (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Error
    """

    return (
        await asyncio_detailed(
            client=client,
            x_push_secret=x_push_secret,
        )
    ).parsed
