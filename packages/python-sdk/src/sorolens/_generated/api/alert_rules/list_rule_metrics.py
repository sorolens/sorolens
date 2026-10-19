from http import HTTPStatus
from typing import Any

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.list_rule_metrics_response_200 import ListRuleMetricsResponse200
from ...types import Response


def _get_kwargs() -> dict[str, Any]:

    _kwargs: dict[str, Any] = {
        "method": "get",
        "url": "/api/v1/rules/metrics",
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ListRuleMetricsResponse200 | None:
    if response.status_code == 200:
        response_200 = ListRuleMetricsResponse200.from_dict(response.json())

        return response_200

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[ListRuleMetricsResponse200]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient,
) -> Response[ListRuleMetricsResponse200]:
    """Rule metric catalog

     Every metric the rule language understands, with its unit and a short
    description, plus the aggregation functions and valid network names.
    The dashboard editor uses this for autocomplete and unit hints.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ListRuleMetricsResponse200]
    """

    kwargs = _get_kwargs()

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient,
) -> ListRuleMetricsResponse200 | None:
    """Rule metric catalog

     Every metric the rule language understands, with its unit and a short
    description, plus the aggregation functions and valid network names.
    The dashboard editor uses this for autocomplete and unit hints.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ListRuleMetricsResponse200
    """

    return sync_detailed(
        client=client,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient,
) -> Response[ListRuleMetricsResponse200]:
    """Rule metric catalog

     Every metric the rule language understands, with its unit and a short
    description, plus the aggregation functions and valid network names.
    The dashboard editor uses this for autocomplete and unit hints.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ListRuleMetricsResponse200]
    """

    kwargs = _get_kwargs()

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient,
) -> ListRuleMetricsResponse200 | None:
    """Rule metric catalog

     Every metric the rule language understands, with its unit and a short
    description, plus the aggregation functions and valid network names.
    The dashboard editor uses this for autocomplete and unit hints.

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ListRuleMetricsResponse200
    """

    return (
        await asyncio_detailed(
            client=client,
        )
    ).parsed
