import datetime
from http import HTTPStatus
from typing import Any

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.failure_category import FailureCategory
from ...models.list_all_invocations_network import ListAllInvocationsNetwork
from ...models.list_all_invocations_response_200 import ListAllInvocationsResponse200
from ...models.list_all_invocations_status import ListAllInvocationsStatus
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    cursor: str | Unset = UNSET,
    limit: int | Unset = 50,
    contract_id: str | Unset = UNSET,
    fn: str | Unset = UNSET,
    status: ListAllInvocationsStatus | Unset = UNSET,
    network: ListAllInvocationsNetwork | Unset = UNSET,
    from_: int | Unset = UNSET,
    to: int | Unset = UNSET,
    since: datetime.datetime | Unset = UNSET,
    until: datetime.datetime | Unset = UNSET,
    failure_category: FailureCategory | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["cursor"] = cursor

    params["limit"] = limit

    params["contract_id"] = contract_id

    params["fn"] = fn

    json_status: str | Unset = UNSET
    if not isinstance(status, Unset):
        json_status = status.value

    params["status"] = json_status

    json_network: str | Unset = UNSET
    if not isinstance(network, Unset):
        json_network = network.value

    params["network"] = json_network

    params["from"] = from_

    params["to"] = to

    json_since: str | Unset = UNSET
    if not isinstance(since, Unset):
        json_since = since.isoformat()
    params["since"] = json_since

    json_until: str | Unset = UNSET
    if not isinstance(until, Unset):
        json_until = until.isoformat()
    params["until"] = json_until

    json_failure_category: str | Unset = UNSET
    if not isinstance(failure_category, Unset):
        json_failure_category = failure_category.value

    params["failure_category"] = json_failure_category

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v1/invocations",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | ListAllInvocationsResponse200 | None:
    if response.status_code == 200:
        response_200 = ListAllInvocationsResponse200.from_dict(response.json())

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
) -> Response[Error | ListAllInvocationsResponse200]:
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
    fn: str | Unset = UNSET,
    status: ListAllInvocationsStatus | Unset = UNSET,
    network: ListAllInvocationsNetwork | Unset = UNSET,
    from_: int | Unset = UNSET,
    to: int | Unset = UNSET,
    since: datetime.datetime | Unset = UNSET,
    until: datetime.datetime | Unset = UNSET,
    failure_category: FailureCategory | Unset = UNSET,
) -> Response[Error | ListAllInvocationsResponse200]:
    """List invocations across all tracked contracts

     Cross-contract invocation explorer, newest first (ledger DESC).

    Args:
        cursor (str | Unset):
        limit (int | Unset):  Default: 50.
        contract_id (str | Unset):
        fn (str | Unset):
        status (ListAllInvocationsStatus | Unset):
        network (ListAllInvocationsNetwork | Unset):
        from_ (int | Unset):
        to (int | Unset):
        since (datetime.datetime | Unset):
        until (datetime.datetime | Unset):
        failure_category (FailureCategory | Unset): Why a failed invocation failed, derived from
            the Soroban host error and the InvokeHostFunction result code (see docs/failure-
            categories.md). Set only on FAILED invocations; omitted for every other status and for
            invocations indexed before classification existed. budget_exceeded: the invocation ran out
            of declared resources (host error type Budget, or result RESOURCE_LIMIT_EXCEEDED /
            INSUFFICIENT_REFUNDABLE_FEE). contract_error: the contract returned its own error code
            (type Contract). auth_failure: authorization failed (type Auth). storage: a ledger entry
            was missing or archived (type Storage, or result ENTRY_ARCHIVED). wasm_trap: the
            contract's Wasm trapped (type WasmVm). other: anything else, including unknown or
            undecodable errors; failure_code keeps the raw value.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | ListAllInvocationsResponse200]
    """

    kwargs = _get_kwargs(
        cursor=cursor,
        limit=limit,
        contract_id=contract_id,
        fn=fn,
        status=status,
        network=network,
        from_=from_,
        to=to,
        since=since,
        until=until,
        failure_category=failure_category,
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
    fn: str | Unset = UNSET,
    status: ListAllInvocationsStatus | Unset = UNSET,
    network: ListAllInvocationsNetwork | Unset = UNSET,
    from_: int | Unset = UNSET,
    to: int | Unset = UNSET,
    since: datetime.datetime | Unset = UNSET,
    until: datetime.datetime | Unset = UNSET,
    failure_category: FailureCategory | Unset = UNSET,
) -> Error | ListAllInvocationsResponse200 | None:
    """List invocations across all tracked contracts

     Cross-contract invocation explorer, newest first (ledger DESC).

    Args:
        cursor (str | Unset):
        limit (int | Unset):  Default: 50.
        contract_id (str | Unset):
        fn (str | Unset):
        status (ListAllInvocationsStatus | Unset):
        network (ListAllInvocationsNetwork | Unset):
        from_ (int | Unset):
        to (int | Unset):
        since (datetime.datetime | Unset):
        until (datetime.datetime | Unset):
        failure_category (FailureCategory | Unset): Why a failed invocation failed, derived from
            the Soroban host error and the InvokeHostFunction result code (see docs/failure-
            categories.md). Set only on FAILED invocations; omitted for every other status and for
            invocations indexed before classification existed. budget_exceeded: the invocation ran out
            of declared resources (host error type Budget, or result RESOURCE_LIMIT_EXCEEDED /
            INSUFFICIENT_REFUNDABLE_FEE). contract_error: the contract returned its own error code
            (type Contract). auth_failure: authorization failed (type Auth). storage: a ledger entry
            was missing or archived (type Storage, or result ENTRY_ARCHIVED). wasm_trap: the
            contract's Wasm trapped (type WasmVm). other: anything else, including unknown or
            undecodable errors; failure_code keeps the raw value.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | ListAllInvocationsResponse200
    """

    return sync_detailed(
        client=client,
        cursor=cursor,
        limit=limit,
        contract_id=contract_id,
        fn=fn,
        status=status,
        network=network,
        from_=from_,
        to=to,
        since=since,
        until=until,
        failure_category=failure_category,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient,
    cursor: str | Unset = UNSET,
    limit: int | Unset = 50,
    contract_id: str | Unset = UNSET,
    fn: str | Unset = UNSET,
    status: ListAllInvocationsStatus | Unset = UNSET,
    network: ListAllInvocationsNetwork | Unset = UNSET,
    from_: int | Unset = UNSET,
    to: int | Unset = UNSET,
    since: datetime.datetime | Unset = UNSET,
    until: datetime.datetime | Unset = UNSET,
    failure_category: FailureCategory | Unset = UNSET,
) -> Response[Error | ListAllInvocationsResponse200]:
    """List invocations across all tracked contracts

     Cross-contract invocation explorer, newest first (ledger DESC).

    Args:
        cursor (str | Unset):
        limit (int | Unset):  Default: 50.
        contract_id (str | Unset):
        fn (str | Unset):
        status (ListAllInvocationsStatus | Unset):
        network (ListAllInvocationsNetwork | Unset):
        from_ (int | Unset):
        to (int | Unset):
        since (datetime.datetime | Unset):
        until (datetime.datetime | Unset):
        failure_category (FailureCategory | Unset): Why a failed invocation failed, derived from
            the Soroban host error and the InvokeHostFunction result code (see docs/failure-
            categories.md). Set only on FAILED invocations; omitted for every other status and for
            invocations indexed before classification existed. budget_exceeded: the invocation ran out
            of declared resources (host error type Budget, or result RESOURCE_LIMIT_EXCEEDED /
            INSUFFICIENT_REFUNDABLE_FEE). contract_error: the contract returned its own error code
            (type Contract). auth_failure: authorization failed (type Auth). storage: a ledger entry
            was missing or archived (type Storage, or result ENTRY_ARCHIVED). wasm_trap: the
            contract's Wasm trapped (type WasmVm). other: anything else, including unknown or
            undecodable errors; failure_code keeps the raw value.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | ListAllInvocationsResponse200]
    """

    kwargs = _get_kwargs(
        cursor=cursor,
        limit=limit,
        contract_id=contract_id,
        fn=fn,
        status=status,
        network=network,
        from_=from_,
        to=to,
        since=since,
        until=until,
        failure_category=failure_category,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient,
    cursor: str | Unset = UNSET,
    limit: int | Unset = 50,
    contract_id: str | Unset = UNSET,
    fn: str | Unset = UNSET,
    status: ListAllInvocationsStatus | Unset = UNSET,
    network: ListAllInvocationsNetwork | Unset = UNSET,
    from_: int | Unset = UNSET,
    to: int | Unset = UNSET,
    since: datetime.datetime | Unset = UNSET,
    until: datetime.datetime | Unset = UNSET,
    failure_category: FailureCategory | Unset = UNSET,
) -> Error | ListAllInvocationsResponse200 | None:
    """List invocations across all tracked contracts

     Cross-contract invocation explorer, newest first (ledger DESC).

    Args:
        cursor (str | Unset):
        limit (int | Unset):  Default: 50.
        contract_id (str | Unset):
        fn (str | Unset):
        status (ListAllInvocationsStatus | Unset):
        network (ListAllInvocationsNetwork | Unset):
        from_ (int | Unset):
        to (int | Unset):
        since (datetime.datetime | Unset):
        until (datetime.datetime | Unset):
        failure_category (FailureCategory | Unset): Why a failed invocation failed, derived from
            the Soroban host error and the InvokeHostFunction result code (see docs/failure-
            categories.md). Set only on FAILED invocations; omitted for every other status and for
            invocations indexed before classification existed. budget_exceeded: the invocation ran out
            of declared resources (host error type Budget, or result RESOURCE_LIMIT_EXCEEDED /
            INSUFFICIENT_REFUNDABLE_FEE). contract_error: the contract returned its own error code
            (type Contract). auth_failure: authorization failed (type Auth). storage: a ledger entry
            was missing or archived (type Storage, or result ENTRY_ARCHIVED). wasm_trap: the
            contract's Wasm trapped (type WasmVm). other: anything else, including unknown or
            undecodable errors; failure_code keeps the raw value.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | ListAllInvocationsResponse200
    """

    return (
        await asyncio_detailed(
            client=client,
            cursor=cursor,
            limit=limit,
            contract_id=contract_id,
            fn=fn,
            status=status,
            network=network,
            from_=from_,
            to=to,
            since=since,
            until=until,
            failure_category=failure_category,
        )
    ).parsed
