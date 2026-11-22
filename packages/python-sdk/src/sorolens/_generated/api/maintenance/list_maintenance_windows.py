from http import HTTPStatus
from typing import Any

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.list_maintenance_windows_response_200 import (
    ListMaintenanceWindowsResponse200,
)
from ...models.list_maintenance_windows_scope import ListMaintenanceWindowsScope
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    scope: ListMaintenanceWindowsScope | Unset = UNSET,
    target_id: str | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    json_scope: str | Unset = UNSET
    if not isinstance(scope, Unset):
        json_scope = scope.value

    params["scope"] = json_scope

    params["target_id"] = target_id

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v1/maintenance-windows",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | ListMaintenanceWindowsResponse200 | None:
    if response.status_code == 200:
        response_200 = ListMaintenanceWindowsResponse200.from_dict(response.json())

        return response_200

    if response.status_code == 500:
        response_500 = Error.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Error | ListMaintenanceWindowsResponse200]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    scope: ListMaintenanceWindowsScope | Unset = UNSET,
    target_id: str | Unset = UNSET,
) -> Response[Error | ListMaintenanceWindowsResponse200]:
    """List maintenance windows

     Returns scheduled maintenance windows, optionally filtered by scope or target ID.

    Args:
        scope (ListMaintenanceWindowsScope | Unset):
        target_id (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | ListMaintenanceWindowsResponse200]
    """

    kwargs = _get_kwargs(
        scope=scope,
        target_id=target_id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    scope: ListMaintenanceWindowsScope | Unset = UNSET,
    target_id: str | Unset = UNSET,
) -> Error | ListMaintenanceWindowsResponse200 | None:
    """List maintenance windows

     Returns scheduled maintenance windows, optionally filtered by scope or target ID.

    Args:
        scope (ListMaintenanceWindowsScope | Unset):
        target_id (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | ListMaintenanceWindowsResponse200
    """

    return sync_detailed(
        client=client,
        scope=scope,
        target_id=target_id,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    scope: ListMaintenanceWindowsScope | Unset = UNSET,
    target_id: str | Unset = UNSET,
) -> Response[Error | ListMaintenanceWindowsResponse200]:
    """List maintenance windows

     Returns scheduled maintenance windows, optionally filtered by scope or target ID.

    Args:
        scope (ListMaintenanceWindowsScope | Unset):
        target_id (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | ListMaintenanceWindowsResponse200]
    """

    kwargs = _get_kwargs(
        scope=scope,
        target_id=target_id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    scope: ListMaintenanceWindowsScope | Unset = UNSET,
    target_id: str | Unset = UNSET,
) -> Error | ListMaintenanceWindowsResponse200 | None:
    """List maintenance windows

     Returns scheduled maintenance windows, optionally filtered by scope or target ID.

    Args:
        scope (ListMaintenanceWindowsScope | Unset):
        target_id (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | ListMaintenanceWindowsResponse200
    """

    return (
        await asyncio_detailed(
            client=client,
            scope=scope,
            target_id=target_id,
        )
    ).parsed
