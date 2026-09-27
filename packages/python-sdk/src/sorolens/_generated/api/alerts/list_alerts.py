from http import HTTPStatus
from typing import Any

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.list_alerts_network import ListAlertsNetwork
from ...models.list_alerts_response_200 import ListAlertsResponse200
from ...models.list_alerts_severity import ListAlertsSeverity
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    flat: bool | Unset = UNSET,
    contract_id: str | Unset = UNSET,
    severity: ListAlertsSeverity | Unset = UNSET,
    network: ListAlertsNetwork | Unset = UNSET,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["flat"] = flat

    params["contract_id"] = contract_id

    json_severity: str | Unset = UNSET
    if not isinstance(severity, Unset):
        json_severity = severity.value

    params["severity"] = json_severity

    json_network: str | Unset = UNSET
    if not isinstance(network, Unset):
        json_network = network.value

    params["network"] = json_network

    params["cursor"] = cursor

    params["limit"] = limit

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v1/alerts",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | ListAlertsResponse200 | None:
    if response.status_code == 200:
        response_200 = ListAlertsResponse200.from_dict(response.json())

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
) -> Response[Error | ListAlertsResponse200]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    flat: bool | Unset = UNSET,
    contract_id: str | Unset = UNSET,
    severity: ListAlertsSeverity | Unset = UNSET,
    network: ListAlertsNetwork | Unset = UNSET,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> Response[Error | ListAlertsResponse200]:
    """List deduplicated alert groups

     Returns the grouped, deduplicated alert view by default (one row per
    group, ordered by last_seen descending). Pass `flat=true` to get the
    raw per-event alert feed instead.

    Args:
        flat (bool | Unset):
        contract_id (str | Unset):
        severity (ListAlertsSeverity | Unset):
        network (ListAlertsNetwork | Unset):
        cursor (str | Unset):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | ListAlertsResponse200]
    """

    kwargs = _get_kwargs(
        flat=flat,
        contract_id=contract_id,
        severity=severity,
        network=network,
        cursor=cursor,
        limit=limit,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    flat: bool | Unset = UNSET,
    contract_id: str | Unset = UNSET,
    severity: ListAlertsSeverity | Unset = UNSET,
    network: ListAlertsNetwork | Unset = UNSET,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> Error | ListAlertsResponse200 | None:
    """List deduplicated alert groups

     Returns the grouped, deduplicated alert view by default (one row per
    group, ordered by last_seen descending). Pass `flat=true` to get the
    raw per-event alert feed instead.

    Args:
        flat (bool | Unset):
        contract_id (str | Unset):
        severity (ListAlertsSeverity | Unset):
        network (ListAlertsNetwork | Unset):
        cursor (str | Unset):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | ListAlertsResponse200
    """

    return sync_detailed(
        client=client,
        flat=flat,
        contract_id=contract_id,
        severity=severity,
        network=network,
        cursor=cursor,
        limit=limit,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    flat: bool | Unset = UNSET,
    contract_id: str | Unset = UNSET,
    severity: ListAlertsSeverity | Unset = UNSET,
    network: ListAlertsNetwork | Unset = UNSET,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> Response[Error | ListAlertsResponse200]:
    """List deduplicated alert groups

     Returns the grouped, deduplicated alert view by default (one row per
    group, ordered by last_seen descending). Pass `flat=true` to get the
    raw per-event alert feed instead.

    Args:
        flat (bool | Unset):
        contract_id (str | Unset):
        severity (ListAlertsSeverity | Unset):
        network (ListAlertsNetwork | Unset):
        cursor (str | Unset):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | ListAlertsResponse200]
    """

    kwargs = _get_kwargs(
        flat=flat,
        contract_id=contract_id,
        severity=severity,
        network=network,
        cursor=cursor,
        limit=limit,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    flat: bool | Unset = UNSET,
    contract_id: str | Unset = UNSET,
    severity: ListAlertsSeverity | Unset = UNSET,
    network: ListAlertsNetwork | Unset = UNSET,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
) -> Error | ListAlertsResponse200 | None:
    """List deduplicated alert groups

     Returns the grouped, deduplicated alert view by default (one row per
    group, ordered by last_seen descending). Pass `flat=true` to get the
    raw per-event alert feed instead.

    Args:
        flat (bool | Unset):
        contract_id (str | Unset):
        severity (ListAlertsSeverity | Unset):
        network (ListAlertsNetwork | Unset):
        cursor (str | Unset):
        limit (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | ListAlertsResponse200
    """

    return (
        await asyncio_detailed(
            client=client,
            flat=flat,
            contract_id=contract_id,
            severity=severity,
            network=network,
            cursor=cursor,
            limit=limit,
        )
    ).parsed
