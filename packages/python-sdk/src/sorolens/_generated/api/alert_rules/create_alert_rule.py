from http import HTTPStatus
from typing import Any, cast

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.alert_rule import AlertRule
from ...models.create_alert_rule_body import CreateAlertRuleBody
from ...models.error import Error
from ...types import Response


def _get_kwargs(
    *,
    body: CreateAlertRuleBody,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/api/v1/rules",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> AlertRule | Any | Error | None:
    if response.status_code == 201:
        response_201 = AlertRule.from_dict(response.json())

        return response_201

    if response.status_code == 409:
        response_409 = Error.from_dict(response.json())

        return response_409

    if response.status_code == 422:
        response_422 = Error.from_dict(response.json())

        return response_422

    if response.status_code == 500:
        response_500 = cast(Any, None)
        return response_500

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[AlertRule | Any | Error]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient,
    body: CreateAlertRuleBody,
) -> Response[AlertRule | Any | Error]:
    """Create an alert rule

     Parses and validates `source` before storing it, so a malformed rule
    can never reach the evaluator. Returns 422 with a structured
    diagnostic (line, column, hint) when the rule does not validate, and
    409 when an identical rule already exists.

    Args:
        body (CreateAlertRuleBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AlertRule | Any | Error]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient,
    body: CreateAlertRuleBody,
) -> AlertRule | Any | Error | None:
    """Create an alert rule

     Parses and validates `source` before storing it, so a malformed rule
    can never reach the evaluator. Returns 422 with a structured
    diagnostic (line, column, hint) when the rule does not validate, and
    409 when an identical rule already exists.

    Args:
        body (CreateAlertRuleBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AlertRule | Any | Error
    """

    return sync_detailed(
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient,
    body: CreateAlertRuleBody,
) -> Response[AlertRule | Any | Error]:
    """Create an alert rule

     Parses and validates `source` before storing it, so a malformed rule
    can never reach the evaluator. Returns 422 with a structured
    diagnostic (line, column, hint) when the rule does not validate, and
    409 when an identical rule already exists.

    Args:
        body (CreateAlertRuleBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[AlertRule | Any | Error]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient,
    body: CreateAlertRuleBody,
) -> AlertRule | Any | Error | None:
    """Create an alert rule

     Parses and validates `source` before storing it, so a malformed rule
    can never reach the evaluator. Returns 422 with a structured
    diagnostic (line, column, hint) when the rule does not validate, and
    409 when an identical rule already exists.

    Args:
        body (CreateAlertRuleBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        AlertRule | Any | Error
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
        )
    ).parsed
