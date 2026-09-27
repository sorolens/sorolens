from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.contract_report import ContractReport
from ...models.error import Error
from ...models.get_contract_report_format import GetContractReportFormat
from ...types import UNSET, Response, Unset


def _get_kwargs(
    contract_id: str,
    *,
    month: str | Unset = UNSET,
    format_: GetContractReportFormat | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["month"] = month

    json_format_: str | Unset = UNSET
    if not isinstance(format_, Unset):
        json_format_ = format_.value

    params["format"] = json_format_

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v1/reports/{contract_id}".format(
            contract_id=quote(str(contract_id), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ContractReport | Error | None:
    if response.status_code == 200:
        response_200 = ContractReport.from_dict(response.json())

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
) -> Response[ContractReport | Error]:
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
    month: str | Unset = UNSET,
    format_: GetContractReportFormat | Unset = UNSET,
) -> Response[ContractReport | Error]:
    """Get a contract's monthly SLA report

     Returns the monthly SLA/uptime report for a contract, derived from
    stored watchdog health checks and alerts. Supports JSON (default),
    CSV, and PDF export formats.

    Args:
        contract_id (str):
        month (str | Unset):
        format_ (GetContractReportFormat | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContractReport | Error]
    """

    kwargs = _get_kwargs(
        contract_id=contract_id,
        month=month,
        format_=format_,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    contract_id: str,
    *,
    client: AuthenticatedClient | Client,
    month: str | Unset = UNSET,
    format_: GetContractReportFormat | Unset = UNSET,
) -> ContractReport | Error | None:
    """Get a contract's monthly SLA report

     Returns the monthly SLA/uptime report for a contract, derived from
    stored watchdog health checks and alerts. Supports JSON (default),
    CSV, and PDF export formats.

    Args:
        contract_id (str):
        month (str | Unset):
        format_ (GetContractReportFormat | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContractReport | Error
    """

    return sync_detailed(
        contract_id=contract_id,
        client=client,
        month=month,
        format_=format_,
    ).parsed


async def asyncio_detailed(
    contract_id: str,
    *,
    client: AuthenticatedClient | Client,
    month: str | Unset = UNSET,
    format_: GetContractReportFormat | Unset = UNSET,
) -> Response[ContractReport | Error]:
    """Get a contract's monthly SLA report

     Returns the monthly SLA/uptime report for a contract, derived from
    stored watchdog health checks and alerts. Supports JSON (default),
    CSV, and PDF export formats.

    Args:
        contract_id (str):
        month (str | Unset):
        format_ (GetContractReportFormat | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContractReport | Error]
    """

    kwargs = _get_kwargs(
        contract_id=contract_id,
        month=month,
        format_=format_,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    contract_id: str,
    *,
    client: AuthenticatedClient | Client,
    month: str | Unset = UNSET,
    format_: GetContractReportFormat | Unset = UNSET,
) -> ContractReport | Error | None:
    """Get a contract's monthly SLA report

     Returns the monthly SLA/uptime report for a contract, derived from
    stored watchdog health checks and alerts. Supports JSON (default),
    CSV, and PDF export formats.

    Args:
        contract_id (str):
        month (str | Unset):
        format_ (GetContractReportFormat | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContractReport | Error
    """

    return (
        await asyncio_detailed(
            contract_id=contract_id,
            client=client,
            month=month,
            format_=format_,
        )
    ).parsed
