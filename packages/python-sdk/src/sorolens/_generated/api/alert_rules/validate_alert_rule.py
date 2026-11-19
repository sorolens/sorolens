from http import HTTPStatus
from typing import Any

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.validate_alert_rule_body import ValidateAlertRuleBody
from ...models.validate_alert_rule_response_200 import ValidateAlertRuleResponse200
from ...types import Response


def _get_kwargs(
    *,
    body: ValidateAlertRuleBody,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/api/v1/rules/validate",
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | ValidateAlertRuleResponse200 | None:
    if response.status_code == 200:
        response_200 = ValidateAlertRuleResponse200.from_dict(response.json())

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
) -> Response[Error | ValidateAlertRuleResponse200]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient,
    body: ValidateAlertRuleBody,
) -> Response[Error | ValidateAlertRuleResponse200]:
    """Validate a rule without storing it

     Parses and checks the rule, returning either the normalized source
    with the metrics it references, or a structured diagnostic. Never
    writes.

    Args:
        body (ValidateAlertRuleBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | ValidateAlertRuleResponse200]
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
    body: ValidateAlertRuleBody,
) -> Error | ValidateAlertRuleResponse200 | None:
    """Validate a rule without storing it

     Parses and checks the rule, returning either the normalized source
    with the metrics it references, or a structured diagnostic. Never
    writes.

    Args:
        body (ValidateAlertRuleBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | ValidateAlertRuleResponse200
    """

    return sync_detailed(
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient,
    body: ValidateAlertRuleBody,
) -> Response[Error | ValidateAlertRuleResponse200]:
    """Validate a rule without storing it

     Parses and checks the rule, returning either the normalized source
    with the metrics it references, or a structured diagnostic. Never
    writes.

    Args:
        body (ValidateAlertRuleBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | ValidateAlertRuleResponse200]
    """

    kwargs = _get_kwargs(
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient,
    body: ValidateAlertRuleBody,
) -> Error | ValidateAlertRuleResponse200 | None:
    """Validate a rule without storing it

     Parses and checks the rule, returning either the normalized source
    with the metrics it references, or a structured diagnostic. Never
    writes.

    Args:
        body (ValidateAlertRuleBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | ValidateAlertRuleResponse200
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
        )
    ).parsed
