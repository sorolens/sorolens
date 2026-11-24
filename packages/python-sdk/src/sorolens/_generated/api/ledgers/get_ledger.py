from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.get_ledger_network import GetLedgerNetwork
from ...models.get_ledger_response_200 import GetLedgerResponse200
from ...types import UNSET, Response, Unset


def _get_kwargs(
    sequence: int,
    *,
    network: GetLedgerNetwork | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    json_network: str | Unset = UNSET
    if not isinstance(network, Unset):
        json_network = network.value

    params["network"] = json_network

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v1/ledgers/{sequence}".format(
            sequence=quote(str(sequence), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | GetLedgerResponse200 | None:
    if response.status_code == 200:
        response_200 = GetLedgerResponse200.from_dict(response.json())

        return response_200

    if response.status_code == 400:
        response_400 = Error.from_dict(response.json())

        return response_400

    if response.status_code == 404:
        response_404 = Error.from_dict(response.json())

        return response_404

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
) -> Response[Error | GetLedgerResponse200]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    sequence: int,
    *,
    client: AuthenticatedClient,
    network: GetLedgerNetwork | Unset = UNSET,
) -> Response[Error | GetLedgerResponse200]:
    """Get ledger summary

     Returns aggregated statistics for a single ledger: close time, transaction count, invocation count,
    failed invocations, event count, and the top contracts by event volume.

    Args:
        sequence (int):
        network (GetLedgerNetwork | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | GetLedgerResponse200]
    """

    kwargs = _get_kwargs(
        sequence=sequence,
        network=network,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    sequence: int,
    *,
    client: AuthenticatedClient,
    network: GetLedgerNetwork | Unset = UNSET,
) -> Error | GetLedgerResponse200 | None:
    """Get ledger summary

     Returns aggregated statistics for a single ledger: close time, transaction count, invocation count,
    failed invocations, event count, and the top contracts by event volume.

    Args:
        sequence (int):
        network (GetLedgerNetwork | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | GetLedgerResponse200
    """

    return sync_detailed(
        sequence=sequence,
        client=client,
        network=network,
    ).parsed


async def asyncio_detailed(
    sequence: int,
    *,
    client: AuthenticatedClient,
    network: GetLedgerNetwork | Unset = UNSET,
) -> Response[Error | GetLedgerResponse200]:
    """Get ledger summary

     Returns aggregated statistics for a single ledger: close time, transaction count, invocation count,
    failed invocations, event count, and the top contracts by event volume.

    Args:
        sequence (int):
        network (GetLedgerNetwork | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | GetLedgerResponse200]
    """

    kwargs = _get_kwargs(
        sequence=sequence,
        network=network,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    sequence: int,
    *,
    client: AuthenticatedClient,
    network: GetLedgerNetwork | Unset = UNSET,
) -> Error | GetLedgerResponse200 | None:
    """Get ledger summary

     Returns aggregated statistics for a single ledger: close time, transaction count, invocation count,
    failed invocations, event count, and the top contracts by event volume.

    Args:
        sequence (int):
        network (GetLedgerNetwork | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | GetLedgerResponse200
    """

    return (
        await asyncio_detailed(
            sequence=sequence,
            client=client,
            network=network,
        )
    ).parsed
