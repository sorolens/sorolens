import datetime
from http import HTTPStatus
from typing import Any

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.list_all_events_network import ListAllEventsNetwork
from ...models.list_all_events_response_200 import ListAllEventsResponse200
from ...models.list_all_events_type import ListAllEventsType
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    cursor: str | Unset = UNSET,
    limit: int | Unset = 50,
    contract_id: str | Unset = UNSET,
    type_: ListAllEventsType | Unset = UNSET,
    network: ListAllEventsNetwork | Unset = UNSET,
    since: datetime.datetime | Unset = UNSET,
    until: datetime.datetime | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["cursor"] = cursor

    params["limit"] = limit

    params["contract_id"] = contract_id

    json_type_: str | Unset = UNSET
    if not isinstance(type_, Unset):
        json_type_ = type_.value

    params["type"] = json_type_

    json_network: str | Unset = UNSET
    if not isinstance(network, Unset):
        json_network = network.value

    params["network"] = json_network

    json_since: str | Unset = UNSET
    if not isinstance(since, Unset):
        json_since = since.isoformat()
    params["since"] = json_since

    json_until: str | Unset = UNSET
    if not isinstance(until, Unset):
        json_until = until.isoformat()
    params["until"] = json_until

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v1/events",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | ListAllEventsResponse200 | None:
    if response.status_code == 200:
        response_200 = ListAllEventsResponse200.from_dict(response.json())

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
) -> Response[Error | ListAllEventsResponse200]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient,
    cursor: str | Unset = UNSET,
    limit: int | Unset = 50,
    contract_id: str | Unset = UNSET,
    type_: ListAllEventsType | Unset = UNSET,
    network: ListAllEventsNetwork | Unset = UNSET,
    since: datetime.datetime | Unset = UNSET,
    until: datetime.datetime | Unset = UNSET,
) -> Response[Error | ListAllEventsResponse200]:
    """List events across all tracked contracts

     Cross-contract events explorer feed, newest first.

    Args:
        cursor (str | Unset):
        limit (int | Unset):  Default: 50.
        contract_id (str | Unset):
        type_ (ListAllEventsType | Unset):
        network (ListAllEventsNetwork | Unset):
        since (datetime.datetime | Unset):
        until (datetime.datetime | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | ListAllEventsResponse200]
    """

    kwargs = _get_kwargs(
        cursor=cursor,
        limit=limit,
        contract_id=contract_id,
        type_=type_,
        network=network,
        since=since,
        until=until,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient,
    cursor: str | Unset = UNSET,
    limit: int | Unset = 50,
    contract_id: str | Unset = UNSET,
    type_: ListAllEventsType | Unset = UNSET,
    network: ListAllEventsNetwork | Unset = UNSET,
    since: datetime.datetime | Unset = UNSET,
    until: datetime.datetime | Unset = UNSET,
) -> Error | ListAllEventsResponse200 | None:
    """List events across all tracked contracts

     Cross-contract events explorer feed, newest first.

    Args:
        cursor (str | Unset):
        limit (int | Unset):  Default: 50.
        contract_id (str | Unset):
        type_ (ListAllEventsType | Unset):
        network (ListAllEventsNetwork | Unset):
        since (datetime.datetime | Unset):
        until (datetime.datetime | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | ListAllEventsResponse200
    """

    return sync_detailed(
        client=client,
        cursor=cursor,
        limit=limit,
        contract_id=contract_id,
        type_=type_,
        network=network,
        since=since,
        until=until,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient,
    cursor: str | Unset = UNSET,
    limit: int | Unset = 50,
    contract_id: str | Unset = UNSET,
    type_: ListAllEventsType | Unset = UNSET,
    network: ListAllEventsNetwork | Unset = UNSET,
    since: datetime.datetime | Unset = UNSET,
    until: datetime.datetime | Unset = UNSET,
) -> Response[Error | ListAllEventsResponse200]:
    """List events across all tracked contracts

     Cross-contract events explorer feed, newest first.

    Args:
        cursor (str | Unset):
        limit (int | Unset):  Default: 50.
        contract_id (str | Unset):
        type_ (ListAllEventsType | Unset):
        network (ListAllEventsNetwork | Unset):
        since (datetime.datetime | Unset):
        until (datetime.datetime | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | ListAllEventsResponse200]
    """

    kwargs = _get_kwargs(
        cursor=cursor,
        limit=limit,
        contract_id=contract_id,
        type_=type_,
        network=network,
        since=since,
        until=until,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient,
    cursor: str | Unset = UNSET,
    limit: int | Unset = 50,
    contract_id: str | Unset = UNSET,
    type_: ListAllEventsType | Unset = UNSET,
    network: ListAllEventsNetwork | Unset = UNSET,
    since: datetime.datetime | Unset = UNSET,
    until: datetime.datetime | Unset = UNSET,
) -> Error | ListAllEventsResponse200 | None:
    """List events across all tracked contracts

     Cross-contract events explorer feed, newest first.

    Args:
        cursor (str | Unset):
        limit (int | Unset):  Default: 50.
        contract_id (str | Unset):
        type_ (ListAllEventsType | Unset):
        network (ListAllEventsNetwork | Unset):
        since (datetime.datetime | Unset):
        until (datetime.datetime | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | ListAllEventsResponse200
    """

    return (
        await asyncio_detailed(
            client=client,
            cursor=cursor,
            limit=limit,
            contract_id=contract_id,
            type_=type_,
            network=network,
            since=since,
            until=until,
        )
    ).parsed
