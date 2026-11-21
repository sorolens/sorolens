from http import HTTPStatus
from typing import Any

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.get_alert_counts_network import GetAlertCountsNetwork
from ...models.get_alert_counts_response_200 import GetAlertCountsResponse200
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    contract_id: str | Unset = UNSET,
    network: GetAlertCountsNetwork | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["contract_id"] = contract_id

    json_network: str | Unset = UNSET
    if not isinstance(network, Unset):
        json_network = network.value

    params["network"] = json_network

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v1/alerts/counts",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | GetAlertCountsResponse200 | None:
    if response.status_code == 200:
        response_200 = GetAlertCountsResponse200.from_dict(response.json())

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
) -> Response[Error | GetAlertCountsResponse200]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    contract_id: str | Unset = UNSET,
    network: GetAlertCountsNetwork | Unset = UNSET,
) -> Response[Error | GetAlertCountsResponse200]:
    """Get alert counts broken down by state

     Returns counts of open, acknowledged, snoozed, and resolved alerts.

    Args:
        contract_id (str | Unset):
        network (GetAlertCountsNetwork | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | GetAlertCountsResponse200]
    """

    kwargs = _get_kwargs(
        contract_id=contract_id,
        network=network,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    contract_id: str | Unset = UNSET,
    network: GetAlertCountsNetwork | Unset = UNSET,
) -> Error | GetAlertCountsResponse200 | None:
    """Get alert counts broken down by state

     Returns counts of open, acknowledged, snoozed, and resolved alerts.

    Args:
        contract_id (str | Unset):
        network (GetAlertCountsNetwork | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | GetAlertCountsResponse200
    """

    return sync_detailed(
        client=client,
        contract_id=contract_id,
        network=network,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    contract_id: str | Unset = UNSET,
    network: GetAlertCountsNetwork | Unset = UNSET,
) -> Response[Error | GetAlertCountsResponse200]:
    """Get alert counts broken down by state

     Returns counts of open, acknowledged, snoozed, and resolved alerts.

    Args:
        contract_id (str | Unset):
        network (GetAlertCountsNetwork | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | GetAlertCountsResponse200]
    """

    kwargs = _get_kwargs(
        contract_id=contract_id,
        network=network,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    contract_id: str | Unset = UNSET,
    network: GetAlertCountsNetwork | Unset = UNSET,
) -> Error | GetAlertCountsResponse200 | None:
    """Get alert counts broken down by state

     Returns counts of open, acknowledged, snoozed, and resolved alerts.

    Args:
        contract_id (str | Unset):
        network (GetAlertCountsNetwork | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | GetAlertCountsResponse200
    """

    return (
        await asyncio_detailed(
            client=client,
            contract_id=contract_id,
            network=network,
        )
    ).parsed
