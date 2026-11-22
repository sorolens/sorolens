from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.error import Error
from ...models.test_integration_id import TestIntegrationId
from ...models.test_integration_response_200 import TestIntegrationResponse200
from ...models.test_integration_response_409 import TestIntegrationResponse409
from ...models.test_integration_response_502 import TestIntegrationResponse502
from ...types import Response


def _get_kwargs(
    id: TestIntegrationId,
) -> dict[str, Any]:

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/api/v1/integrations/{id}/test".format(
            id=quote(str(id), safe=""),
        ),
    }

    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> (
    Error
    | TestIntegrationResponse200
    | TestIntegrationResponse409
    | TestIntegrationResponse502
    | None
):
    if response.status_code == 200:
        response_200 = TestIntegrationResponse200.from_dict(response.json())

        return response_200

    if response.status_code == 404:
        response_404 = Error.from_dict(response.json())

        return response_404

    if response.status_code == 409:
        response_409 = TestIntegrationResponse409.from_dict(response.json())

        return response_409

    if response.status_code == 502:
        response_502 = TestIntegrationResponse502.from_dict(response.json())

        return response_502

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[
    Error
    | TestIntegrationResponse200
    | TestIntegrationResponse409
    | TestIntegrationResponse502
]:
    return Response(
        status_code=HTTPStatus(response.status_code),
        content=response.content,
        headers=response.headers,
        parsed=_parse_response(client=client, response=response),
    )


def sync_detailed(
    id: TestIntegrationId,
    *,
    client: AuthenticatedClient,
) -> Response[
    Error
    | TestIntegrationResponse200
    | TestIntegrationResponse409
    | TestIntegrationResponse502
]:
    """Send a test message through an integration

     Sends a real test message through the integration's configured channel
    so users can verify their setup end to end (issue #612). Returns 409
    when the integration is not configured and 502 when the remote endpoint
    rejects the message.

    Args:
        id (TestIntegrationId):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | TestIntegrationResponse200 | TestIntegrationResponse409 | TestIntegrationResponse502]
    """

    kwargs = _get_kwargs(
        id=id,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    id: TestIntegrationId,
    *,
    client: AuthenticatedClient,
) -> (
    Error
    | TestIntegrationResponse200
    | TestIntegrationResponse409
    | TestIntegrationResponse502
    | None
):
    """Send a test message through an integration

     Sends a real test message through the integration's configured channel
    so users can verify their setup end to end (issue #612). Returns 409
    when the integration is not configured and 502 when the remote endpoint
    rejects the message.

    Args:
        id (TestIntegrationId):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | TestIntegrationResponse200 | TestIntegrationResponse409 | TestIntegrationResponse502
    """

    return sync_detailed(
        id=id,
        client=client,
    ).parsed


async def asyncio_detailed(
    id: TestIntegrationId,
    *,
    client: AuthenticatedClient,
) -> Response[
    Error
    | TestIntegrationResponse200
    | TestIntegrationResponse409
    | TestIntegrationResponse502
]:
    """Send a test message through an integration

     Sends a real test message through the integration's configured channel
    so users can verify their setup end to end (issue #612). Returns 409
    when the integration is not configured and 502 when the remote endpoint
    rejects the message.

    Args:
        id (TestIntegrationId):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[Error | TestIntegrationResponse200 | TestIntegrationResponse409 | TestIntegrationResponse502]
    """

    kwargs = _get_kwargs(
        id=id,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    id: TestIntegrationId,
    *,
    client: AuthenticatedClient,
) -> (
    Error
    | TestIntegrationResponse200
    | TestIntegrationResponse409
    | TestIntegrationResponse502
    | None
):
    """Send a test message through an integration

     Sends a real test message through the integration's configured channel
    so users can verify their setup end to end (issue #612). Returns 409
    when the integration is not configured and 502 when the remote endpoint
    rejects the message.

    Args:
        id (TestIntegrationId):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Error | TestIntegrationResponse200 | TestIntegrationResponse409 | TestIntegrationResponse502
    """

    return (
        await asyncio_detailed(
            id=id,
            client=client,
        )
    ).parsed
