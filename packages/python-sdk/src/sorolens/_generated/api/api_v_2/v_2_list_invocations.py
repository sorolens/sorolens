from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.v2_invocation_list import V2InvocationList
from ...models.v2_list_invocations_network import V2ListInvocationsNetwork
from ...models.v2_list_invocations_status import V2ListInvocationsStatus
from ...types import UNSET, Response, Unset


def _get_kwargs(
    id: str,
    *,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    network: V2ListInvocationsNetwork | Unset = UNSET,
    status: V2ListInvocationsStatus | Unset = UNSET,
    function_name: str | Unset = UNSET,
    from_: int | Unset = UNSET,
    to: int | Unset = UNSET,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["cursor"] = cursor

    params["limit"] = limit

    json_network: str | Unset = UNSET
    if not isinstance(network, Unset):
        json_network = network.value

    params["network"] = json_network

    json_status: str | Unset = UNSET
    if not isinstance(status, Unset):
        json_status = status.value

    params["status"] = json_status

    params["function_name"] = function_name

    params["from"] = from_

    params["to"] = to

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v2/contracts/{id}/invocations".format(
            id=quote(str(id), safe=""),
        ),
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | V2InvocationList | None:
    if response.status_code == 200:
        response_200 = V2InvocationList.from_dict(response.json())

        return response_200

    if response.status_code == 429:
        response_429 = Error.from_dict(response.json())

        return response_429

    if response.status_code == 500:
        response_500 = Error.from_dict(response.json())

        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Error | V2InvocationList]:
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
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    network: V2ListInvocationsNetwork | Unset = UNSET,
    status: V2ListInvocationsStatus | Unset = UNSET,
    function_name: str | Unset = UNSET,
    from_: int | Unset = UNSET,
    to: int | Unset = UNSET,
) -> Response[Error | V2InvocationList]:
    """List a contract's invocations

    Args:
        id (str):
        cursor (str | Unset):
        limit (int | Unset):
        network (V2ListInvocationsNetwork | Unset):
        status (V2ListInvocationsStatus | Unset):
        function_name (str | Unset):
        from_ (int | Unset):
        to (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | V2InvocationList]
    """

    kwargs = _get_kwargs(
        id=id,
        cursor=cursor,
        limit=limit,
        network=network,
        status=status,
        function_name=function_name,
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
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    network: V2ListInvocationsNetwork | Unset = UNSET,
    status: V2ListInvocationsStatus | Unset = UNSET,
    function_name: str | Unset = UNSET,
    from_: int | Unset = UNSET,
    to: int | Unset = UNSET,
) -> Error | V2InvocationList | None:
    """List a contract's invocations

    Args:
        id (str):
        cursor (str | Unset):
        limit (int | Unset):
        network (V2ListInvocationsNetwork | Unset):
        status (V2ListInvocationsStatus | Unset):
        function_name (str | Unset):
        from_ (int | Unset):
        to (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | V2InvocationList
    """

    return sync_detailed(
        id=id,
        client=client,
        cursor=cursor,
        limit=limit,
        network=network,
        status=status,
        function_name=function_name,
        from_=from_,
        to=to,
    ).parsed


async def asyncio_detailed(
    id: str,
    *,
    client: AuthenticatedClient,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    network: V2ListInvocationsNetwork | Unset = UNSET,
    status: V2ListInvocationsStatus | Unset = UNSET,
    function_name: str | Unset = UNSET,
    from_: int | Unset = UNSET,
    to: int | Unset = UNSET,
) -> Response[Error | V2InvocationList]:
    """List a contract's invocations

    Args:
        id (str):
        cursor (str | Unset):
        limit (int | Unset):
        network (V2ListInvocationsNetwork | Unset):
        status (V2ListInvocationsStatus | Unset):
        function_name (str | Unset):
        from_ (int | Unset):
        to (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | V2InvocationList]
    """

    kwargs = _get_kwargs(
        id=id,
        cursor=cursor,
        limit=limit,
        network=network,
        status=status,
        function_name=function_name,
        from_=from_,
        to=to,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    id: str,
    *,
    client: AuthenticatedClient,
    cursor: str | Unset = UNSET,
    limit: int | Unset = UNSET,
    network: V2ListInvocationsNetwork | Unset = UNSET,
    status: V2ListInvocationsStatus | Unset = UNSET,
    function_name: str | Unset = UNSET,
    from_: int | Unset = UNSET,
    to: int | Unset = UNSET,
) -> Error | V2InvocationList | None:
    """List a contract's invocations

    Args:
        id (str):
        cursor (str | Unset):
        limit (int | Unset):
        network (V2ListInvocationsNetwork | Unset):
        status (V2ListInvocationsStatus | Unset):
        function_name (str | Unset):
        from_ (int | Unset):
        to (int | Unset):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | V2InvocationList
    """

    return (
        await asyncio_detailed(
            id=id,
            client=client,
            cursor=cursor,
            limit=limit,
            network=network,
            status=status,
            function_name=function_name,
            from_=from_,
            to=to,
        )
    ).parsed
