from http import HTTPStatus
from typing import Any

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.slack_command_body import SlackCommandBody
from ...models.slack_message import SlackMessage
from ...types import Response


def _get_kwargs(
    *,
    body: SlackCommandBody,
    x_slack_signature: str,
    x_slack_request_timestamp: str,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}
    headers["X-Slack-Signature"] = x_slack_signature

    headers["X-Slack-Request-Timestamp"] = x_slack_request_timestamp

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/integrations/slack/commands",
    }

    _kwargs["data"] = body.to_dict()
    headers["Content-Type"] = "application/x-www-form-urlencoded"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Error | SlackMessage | None:
    if response.status_code == 200:
        response_200 = SlackMessage.from_dict(response.json())

        return response_200

    if response.status_code == 400:
        response_400 = Error.from_dict(response.json())

        return response_400

    if response.status_code == 401:
        response_401 = Error.from_dict(response.json())

        return response_401

    if response.status_code == 404:
        response_404 = Error.from_dict(response.json())

        return response_404

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[Error | SlackMessage]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: SlackCommandBody,
    x_slack_signature: str,
    x_slack_request_timestamp: str,
) -> Response[Error | SlackMessage]:
    """Slack slash command

     Endpoint for a Slack slash command such as `/sorolens <contract_id>`.
    Every request must carry a valid Slack signature
    (`X-Slack-Signature`, HMAC-SHA256 over `v0:<timestamp>:<body>` with the
    app's signing secret) and an `X-Slack-Request-Timestamp` within five
    minutes. Replies with an ephemeral Block Kit message showing the
    contract's watchdog status and latest alerts. Disabled (404) unless
    `SLACK_SIGNING_SECRET` is set.

    Args:
        x_slack_signature (str):  Example:
            v0=a2114d57b48eac39b9ad189dd8316235a7b4a8d21a10bd27519666489c69b503.
        x_slack_request_timestamp (str):  Example: 1531420618.
        body (SlackCommandBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | SlackMessage]
    """

    kwargs = _get_kwargs(
        body=body,
        x_slack_signature=x_slack_signature,
        x_slack_request_timestamp=x_slack_request_timestamp,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    *,
    client: AuthenticatedClient | Client,
    body: SlackCommandBody,
    x_slack_signature: str,
    x_slack_request_timestamp: str,
) -> Error | SlackMessage | None:
    """Slack slash command

     Endpoint for a Slack slash command such as `/sorolens <contract_id>`.
    Every request must carry a valid Slack signature
    (`X-Slack-Signature`, HMAC-SHA256 over `v0:<timestamp>:<body>` with the
    app's signing secret) and an `X-Slack-Request-Timestamp` within five
    minutes. Replies with an ephemeral Block Kit message showing the
    contract's watchdog status and latest alerts. Disabled (404) unless
    `SLACK_SIGNING_SECRET` is set.

    Args:
        x_slack_signature (str):  Example:
            v0=a2114d57b48eac39b9ad189dd8316235a7b4a8d21a10bd27519666489c69b503.
        x_slack_request_timestamp (str):  Example: 1531420618.
        body (SlackCommandBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | SlackMessage
    """

    return sync_detailed(
        client=client,
        body=body,
        x_slack_signature=x_slack_signature,
        x_slack_request_timestamp=x_slack_request_timestamp,
    ).parsed


async def asyncio_detailed(
    *,
    client: AuthenticatedClient | Client,
    body: SlackCommandBody,
    x_slack_signature: str,
    x_slack_request_timestamp: str,
) -> Response[Error | SlackMessage]:
    """Slack slash command

     Endpoint for a Slack slash command such as `/sorolens <contract_id>`.
    Every request must carry a valid Slack signature
    (`X-Slack-Signature`, HMAC-SHA256 over `v0:<timestamp>:<body>` with the
    app's signing secret) and an `X-Slack-Request-Timestamp` within five
    minutes. Replies with an ephemeral Block Kit message showing the
    contract's watchdog status and latest alerts. Disabled (404) unless
    `SLACK_SIGNING_SECRET` is set.

    Args:
        x_slack_signature (str):  Example:
            v0=a2114d57b48eac39b9ad189dd8316235a7b4a8d21a10bd27519666489c69b503.
        x_slack_request_timestamp (str):  Example: 1531420618.
        body (SlackCommandBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | SlackMessage]
    """

    kwargs = _get_kwargs(
        body=body,
        x_slack_signature=x_slack_signature,
        x_slack_request_timestamp=x_slack_request_timestamp,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    *,
    client: AuthenticatedClient | Client,
    body: SlackCommandBody,
    x_slack_signature: str,
    x_slack_request_timestamp: str,
) -> Error | SlackMessage | None:
    """Slack slash command

     Endpoint for a Slack slash command such as `/sorolens <contract_id>`.
    Every request must carry a valid Slack signature
    (`X-Slack-Signature`, HMAC-SHA256 over `v0:<timestamp>:<body>` with the
    app's signing secret) and an `X-Slack-Request-Timestamp` within five
    minutes. Replies with an ephemeral Block Kit message showing the
    contract's watchdog status and latest alerts. Disabled (404) unless
    `SLACK_SIGNING_SECRET` is set.

    Args:
        x_slack_signature (str):  Example:
            v0=a2114d57b48eac39b9ad189dd8316235a7b4a8d21a10bd27519666489c69b503.
        x_slack_request_timestamp (str):  Example: 1531420618.
        body (SlackCommandBody):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | SlackMessage
    """

    return (
        await asyncio_detailed(
            client=client,
            body=body,
            x_slack_signature=x_slack_signature,
            x_slack_request_timestamp=x_slack_request_timestamp,
        )
    ).parsed
