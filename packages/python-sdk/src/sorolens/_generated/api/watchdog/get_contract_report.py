from http import HTTPStatus
from typing import Any, cast
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
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
) -> Any | Error | None:
    if response.status_code == 200:
        response_200 = cast(Any, None)
        return response_200

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
) -> Response[Any | Error]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    contract_id: str,
    *,
    client: AuthenticatedClient,
    month: str | Unset = UNSET,
    format_: GetContractReportFormat | Unset = UNSET,
) -> Response[Any | Error]:
    """Get the monthly SLA report for a contract

    Args:
        contract_id (str):
        month (str | Unset):
        format_ (GetContractReportFormat | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Error]
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
    client: AuthenticatedClient,
    month: str | Unset = UNSET,
    format_: GetContractReportFormat | Unset = UNSET,
) -> Any | Error | None:
    """Get the monthly SLA report for a contract

    Args:
        contract_id (str):
        month (str | Unset):
        format_ (GetContractReportFormat | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Error
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
    client: AuthenticatedClient,
    month: str | Unset = UNSET,
    format_: GetContractReportFormat | Unset = UNSET,
) -> Response[Any | Error]:
    """Get the monthly SLA report for a contract

    Args:
        contract_id (str):
        month (str | Unset):
        format_ (GetContractReportFormat | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Any | Error]
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
    client: AuthenticatedClient,
    month: str | Unset = UNSET,
    format_: GetContractReportFormat | Unset = UNSET,
) -> Any | Error | None:
    """Get the monthly SLA report for a contract

    Args:
        contract_id (str):
        month (str | Unset):
        format_ (GetContractReportFormat | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Any | Error
    """

    return (
        await asyncio_detailed(
            contract_id=contract_id,
            client=client,
            month=month,
            format_=format_,
        )
    ).parsed
