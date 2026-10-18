from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.list_contract_alerts_network import ListContractAlertsNetwork
from ...models.list_contract_alerts_response_200 import ListContractAlertsResponse200
from ...models.list_contract_alerts_severity import ListContractAlertsSeverity
from ...types import UNSET, Response, Unset


def _get_kwargs(
    id: str,
    *,
    severity: ListContractAlertsSeverity | Unset = UNSET,
    network: ListContractAlertsNetwork | Unset = UNSET,
    limit: int | Unset = UNSET,
    cursor: str | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    json_severity: str | Unset = UNSET
    if not isinstance(severity, Unset):
        json_severity = severity.value

    params["severity"] = json_severity

    json_network: str | Unset = UNSET
    if not isinstance(network, Unset):
        json_network = network.value

    params["network"] = json_network

    params["limit"] = limit

    params["cursor"] = cursor

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v1/watchdog/contracts/{id}/alerts".format(
            id=quote(str(id), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | ListContractAlertsResponse200 | None:
    if response.status_code == 200:
        response_200 = ListContractAlertsResponse200.from_dict(response.json())

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
) -> Response[Error | ListContractAlertsResponse200]:
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
    severity: ListContractAlertsSeverity | Unset = UNSET,
    network: ListContractAlertsNetwork | Unset = UNSET,
    limit: int | Unset = UNSET,
    cursor: str | Unset = UNSET,
) -> Response[Error | ListContractAlertsResponse200]:
    """List alerts for a monitored contract

    Args:
        id (str):
        severity (ListContractAlertsSeverity | Unset):
        network (ListContractAlertsNetwork | Unset):
        limit (int | Unset):
        cursor (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | ListContractAlertsResponse200]
    """

    kwargs = _get_kwargs(
        id=id,
        severity=severity,
        network=network,
        limit=limit,
        cursor=cursor,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    id: str,
    *,
    client: AuthenticatedClient,
    severity: ListContractAlertsSeverity | Unset = UNSET,
    network: ListContractAlertsNetwork | Unset = UNSET,
    limit: int | Unset = UNSET,
    cursor: str | Unset = UNSET,
) -> Error | ListContractAlertsResponse200 | None:
    """List alerts for a monitored contract

    Args:
        id (str):
        severity (ListContractAlertsSeverity | Unset):
        network (ListContractAlertsNetwork | Unset):
        limit (int | Unset):
        cursor (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | ListContractAlertsResponse200
    """

    return sync_detailed(
        id=id,
        client=client,
        severity=severity,
        network=network,
        limit=limit,
        cursor=cursor,
    ).parsed


async def asyncio_detailed(
    id: str,
    *,
    client: AuthenticatedClient,
    severity: ListContractAlertsSeverity | Unset = UNSET,
    network: ListContractAlertsNetwork | Unset = UNSET,
    limit: int | Unset = UNSET,
    cursor: str | Unset = UNSET,
) -> Response[Error | ListContractAlertsResponse200]:
    """List alerts for a monitored contract

    Args:
        id (str):
        severity (ListContractAlertsSeverity | Unset):
        network (ListContractAlertsNetwork | Unset):
        limit (int | Unset):
        cursor (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | ListContractAlertsResponse200]
    """

    kwargs = _get_kwargs(
        id=id,
        severity=severity,
        network=network,
        limit=limit,
        cursor=cursor,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    id: str,
    *,
    client: AuthenticatedClient,
    severity: ListContractAlertsSeverity | Unset = UNSET,
    network: ListContractAlertsNetwork | Unset = UNSET,
    limit: int | Unset = UNSET,
    cursor: str | Unset = UNSET,
) -> Error | ListContractAlertsResponse200 | None:
    """List alerts for a monitored contract

    Args:
        id (str):
        severity (ListContractAlertsSeverity | Unset):
        network (ListContractAlertsNetwork | Unset):
        limit (int | Unset):
        cursor (str | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | ListContractAlertsResponse200
    """

    return (
        await asyncio_detailed(
            id=id,
            client=client,
            severity=severity,
            network=network,
            limit=limit,
            cursor=cursor,
        )
    ).parsed
