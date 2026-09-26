from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.get_contract_report_history_response_200 import (
    GetContractReportHistoryResponse200,
)
from ...types import UNSET, Response, Unset


def _get_kwargs(
    contract_id: str,
    *,
    months: int | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["months"] = months

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v1/reports/{contract_id}/history".format(
            contract_id=quote(str(contract_id), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | GetContractReportHistoryResponse200 | None:
    if response.status_code == 200:
        response_200 = GetContractReportHistoryResponse200.from_dict(response.json())

        return response_200

    if response.status_code == 422:
        response_422 = Error.from_dict(response.json())

        return response_422

    if response.status_code == 500:
        response_500 = Error.from_dict(response.json())

        return response_500

    if response.status_code == 501:
        response_501 = Error.from_dict(response.json())

        return response_501

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Error | GetContractReportHistoryResponse200]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    contract_id: str,
    *,
    client: AuthenticatedClient | Client,
    months: int | Unset = UNSET,
) -> Response[Error | GetContractReportHistoryResponse200]:
    """Get a contract's SLA history across months

    Args:
        contract_id (str):
        months (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | GetContractReportHistoryResponse200]
    """

    kwargs = _get_kwargs(
        contract_id=contract_id,
        months=months,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    contract_id: str,
    *,
    client: AuthenticatedClient | Client,
    months: int | Unset = UNSET,
) -> Error | GetContractReportHistoryResponse200 | None:
    """Get a contract's SLA history across months

    Args:
        contract_id (str):
        months (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | GetContractReportHistoryResponse200
    """

    return sync_detailed(
        contract_id=contract_id,
        client=client,
        months=months,
    ).parsed


async def asyncio_detailed(
    contract_id: str,
    *,
    client: AuthenticatedClient | Client,
    months: int | Unset = UNSET,
) -> Response[Error | GetContractReportHistoryResponse200]:
    """Get a contract's SLA history across months

    Args:
        contract_id (str):
        months (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | GetContractReportHistoryResponse200]
    """

    kwargs = _get_kwargs(
        contract_id=contract_id,
        months=months,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    contract_id: str,
    *,
    client: AuthenticatedClient | Client,
    months: int | Unset = UNSET,
) -> Error | GetContractReportHistoryResponse200 | None:
    """Get a contract's SLA history across months

    Args:
        contract_id (str):
        months (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | GetContractReportHistoryResponse200
    """

    return (
        await asyncio_detailed(
            contract_id=contract_id,
            client=client,
            months=months,
        )
    ).parsed
