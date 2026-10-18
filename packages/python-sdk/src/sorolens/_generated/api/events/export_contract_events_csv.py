from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.export_contract_events_csv_network import ExportContractEventsCsvNetwork
from ...types import UNSET, Response, Unset


def _get_kwargs(
    id: str,
    *,
    network: ExportContractEventsCsvNetwork | Unset = UNSET,
    type_: str | Unset = UNSET,
    from_: int | Unset = UNSET,
    to: int | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    json_network: str | Unset = UNSET
    if not isinstance(network, Unset):
        json_network = network.value

    params["network"] = json_network

    params["type"] = type_

    params["from"] = from_

    params["to"] = to

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v1/contracts/{id}/events.csv".format(
            id=quote(str(id), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | str | None:
    if response.status_code == 200:
        response_200 = response.text
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
) -> Response[Error | str]:
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
    network: ExportContractEventsCsvNetwork | Unset = UNSET,
    type_: str | Unset = UNSET,
    from_: int | Unset = UNSET,
    to: int | Unset = UNSET,
) -> Response[Error | str]:
    """Export contract events as CSV

     Streams every matching event as CSV instead of paginating them, so a large contract can be handed to
    a spreadsheet or an analyst in one request. Rows are ordered by ledger ascending, then event id, and
    the file is deterministic for an unchanged store.
    Unlike the JSON listing there is no cursor and no row cap: an export is meant to be complete. The
    same `type`, `from` and `to` filters apply, but `topic` and `in_successful_call` are not supported
    here.
    The header row is always present, so an unknown contract yields a file with no data rows rather than
    a 404. Free-text columns are prefixed with an apostrophe when they start with a character a
    spreadsheet would treat as a formula.

    Args:
        id (str):
        network (ExportContractEventsCsvNetwork | Unset):
        type_ (str | Unset):
        from_ (int | Unset):
        to (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | str]
    """

    kwargs = _get_kwargs(
        id=id,
        network=network,
        type_=type_,
        from_=from_,
        to=to,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    id: str,
    *,
    client: AuthenticatedClient,
    network: ExportContractEventsCsvNetwork | Unset = UNSET,
    type_: str | Unset = UNSET,
    from_: int | Unset = UNSET,
    to: int | Unset = UNSET,
) -> Error | str | None:
    """Export contract events as CSV

     Streams every matching event as CSV instead of paginating them, so a large contract can be handed to
    a spreadsheet or an analyst in one request. Rows are ordered by ledger ascending, then event id, and
    the file is deterministic for an unchanged store.
    Unlike the JSON listing there is no cursor and no row cap: an export is meant to be complete. The
    same `type`, `from` and `to` filters apply, but `topic` and `in_successful_call` are not supported
    here.
    The header row is always present, so an unknown contract yields a file with no data rows rather than
    a 404. Free-text columns are prefixed with an apostrophe when they start with a character a
    spreadsheet would treat as a formula.

    Args:
        id (str):
        network (ExportContractEventsCsvNetwork | Unset):
        type_ (str | Unset):
        from_ (int | Unset):
        to (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | str
    """

    return sync_detailed(
        id=id,
        client=client,
        network=network,
        type_=type_,
        from_=from_,
        to=to,
    ).parsed


async def asyncio_detailed(
    id: str,
    *,
    client: AuthenticatedClient,
    network: ExportContractEventsCsvNetwork | Unset = UNSET,
    type_: str | Unset = UNSET,
    from_: int | Unset = UNSET,
    to: int | Unset = UNSET,
) -> Response[Error | str]:
    """Export contract events as CSV

     Streams every matching event as CSV instead of paginating them, so a large contract can be handed to
    a spreadsheet or an analyst in one request. Rows are ordered by ledger ascending, then event id, and
    the file is deterministic for an unchanged store.
    Unlike the JSON listing there is no cursor and no row cap: an export is meant to be complete. The
    same `type`, `from` and `to` filters apply, but `topic` and `in_successful_call` are not supported
    here.
    The header row is always present, so an unknown contract yields a file with no data rows rather than
    a 404. Free-text columns are prefixed with an apostrophe when they start with a character a
    spreadsheet would treat as a formula.

    Args:
        id (str):
        network (ExportContractEventsCsvNetwork | Unset):
        type_ (str | Unset):
        from_ (int | Unset):
        to (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | str]
    """

    kwargs = _get_kwargs(
        id=id,
        network=network,
        type_=type_,
        from_=from_,
        to=to,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    id: str,
    *,
    client: AuthenticatedClient,
    network: ExportContractEventsCsvNetwork | Unset = UNSET,
    type_: str | Unset = UNSET,
    from_: int | Unset = UNSET,
    to: int | Unset = UNSET,
) -> Error | str | None:
    """Export contract events as CSV

     Streams every matching event as CSV instead of paginating them, so a large contract can be handed to
    a spreadsheet or an analyst in one request. Rows are ordered by ledger ascending, then event id, and
    the file is deterministic for an unchanged store.
    Unlike the JSON listing there is no cursor and no row cap: an export is meant to be complete. The
    same `type`, `from` and `to` filters apply, but `topic` and `in_successful_call` are not supported
    here.
    The header row is always present, so an unknown contract yields a file with no data rows rather than
    a 404. Free-text columns are prefixed with an apostrophe when they start with a character a
    spreadsheet would treat as a formula.

    Args:
        id (str):
        network (ExportContractEventsCsvNetwork | Unset):
        type_ (str | Unset):
        from_ (int | Unset):
        to (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | str
    """

    return (
        await asyncio_detailed(
            id=id,
            client=client,
            network=network,
            type_=type_,
            from_=from_,
            to=to,
        )
    ).parsed
