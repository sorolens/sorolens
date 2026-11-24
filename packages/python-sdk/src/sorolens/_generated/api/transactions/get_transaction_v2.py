from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.get_transaction_v2_response_200 import GetTransactionV2Response200
from ...types import Response


def _get_kwargs(
    hash_: str,
) -> dict[str, Any]:

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v2/transactions/{hash_}".format(
            hash_=quote(str(hash_), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | GetTransactionV2Response200 | None:
    if response.status_code == 200:
        response_200 = GetTransactionV2Response200.from_dict(response.json())

        return response_200

    if response.status_code == 400:
        response_400 = Error.from_dict(response.json())

        return response_400

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
) -> Response[Error | GetTransactionV2Response200]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    hash_: str,
    *,
    client: AuthenticatedClient,
) -> Response[Error | GetTransactionV2Response200]:
    """Get transaction details (v2)

     v2 passthrough — identical shape to v1.

    Args:
        hash_ (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | GetTransactionV2Response200]
    """

    kwargs = _get_kwargs(
        hash_=hash_,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    hash_: str,
    *,
    client: AuthenticatedClient,
) -> Error | GetTransactionV2Response200 | None:
    """Get transaction details (v2)

     v2 passthrough — identical shape to v1.

    Args:
        hash_ (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | GetTransactionV2Response200
    """

    return sync_detailed(
        hash_=hash_,
        client=client,
    ).parsed


async def asyncio_detailed(
    hash_: str,
    *,
    client: AuthenticatedClient,
) -> Response[Error | GetTransactionV2Response200]:
    """Get transaction details (v2)

     v2 passthrough — identical shape to v1.

    Args:
        hash_ (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | GetTransactionV2Response200]
    """

    kwargs = _get_kwargs(
        hash_=hash_,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    hash_: str,
    *,
    client: AuthenticatedClient,
) -> Error | GetTransactionV2Response200 | None:
    """Get transaction details (v2)

     v2 passthrough — identical shape to v1.

    Args:
        hash_ (str):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | GetTransactionV2Response200
    """

    return (
        await asyncio_detailed(
            hash_=hash_,
            client=client,
        )
    ).parsed
