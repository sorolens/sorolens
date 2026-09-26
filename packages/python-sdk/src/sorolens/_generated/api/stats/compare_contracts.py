from http import HTTPStatus
from typing import Any

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.compare_contracts_window import CompareContractsWindow
from ...models.compare_response import CompareResponse
from ...models.error import Error
from ...types import UNSET, Response, Unset


def _get_kwargs(
    *,
    ids: str,
    window: CompareContractsWindow | Unset = CompareContractsWindow.VALUE_0,
) -> dict[str, Any]:

    params: dict[str, Any] = {}

    params["ids"] = ids

    json_window: str | Unset = UNSET
    if not isinstance(window, Unset):
        json_window = window.value

    params["window"] = json_window

    params = {k: v for k, v in params.items() if v is not UNSET and v is not None}

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v1/compare",
        "params": params,
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> CompareResponse | Error | None:
    if response.status_code == 200:
        response_200 = CompareResponse.from_dict(response.json())

        return response_200

    if response.status_code == 422:
        response_422 = Error.from_dict(response.json())

        return response_422

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[CompareResponse | Error]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient,
    ids: str,
    window: CompareContractsWindow | Unset = CompareContractsWindow.VALUE_0,
) -> Response[CompareResponse | Error]:
    """Compare up to four contracts side by side

    Args:
        ids (str):
        window (CompareContractsWindow | Unset):  Default: CompareContractsWindow.VALUE_0.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[CompareResponse | Error]
    """

    kwargs = _get_kwargs(
        ids=ids,
        window=window,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient,
    ids: str,
    window: CompareContractsWindow | Unset = CompareContractsWindow.VALUE_0,
) -> CompareResponse | Error | None:
    """Compare up to four contracts side by side

    Args:
        ids (str):
        window (CompareContractsWindow | Unset):  Default: CompareContractsWindow.VALUE_0.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        CompareResponse | Error
    """

    return sync_detailed(
        client=client,
        ids=ids,
        window=window,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient,
    ids: str,
    window: CompareContractsWindow | Unset = CompareContractsWindow.VALUE_0,
) -> Response[CompareResponse | Error]:
    """Compare up to four contracts side by side

    Args:
        ids (str):
        window (CompareContractsWindow | Unset):  Default: CompareContractsWindow.VALUE_0.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[CompareResponse | Error]
    """

    kwargs = _get_kwargs(
        ids=ids,
        window=window,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient,
    ids: str,
    window: CompareContractsWindow | Unset = CompareContractsWindow.VALUE_0,
) -> CompareResponse | Error | None:
    """Compare up to four contracts side by side

    Args:
        ids (str):
        window (CompareContractsWindow | Unset):  Default: CompareContractsWindow.VALUE_0.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        CompareResponse | Error
    """

    return (
        await asyncio_detailed(
            client=client,
            ids=ids,
            window=window,
        )
    ).parsed
