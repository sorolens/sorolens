from http import HTTPStatus
from typing import Any
from urllib.parse import quote

import httpx

from ... import errors
from ...client import AuthenticatedClient, Client
from ...models.contract_verification import ContractVerification
from ...models.contract_verification_request import ContractVerificationRequest
from ...models.error import Error
from ...types import Response


def _get_kwargs(
    id: str,
    *,
    body: ContractVerificationRequest,
) -> dict[str, Any]:
    headers: dict[str, Any] = {}

    _kwargs: dict[str, Any] = {
        "method": "post",
        "url": "/api/v1/contracts/{id}/verify".format(
            id=quote(str(id), safe=""),
        ),
    }

    _kwargs["json"] = body.to_dict()

    headers["Content-Type"] = "application/json"

    _kwargs["headers"] = headers
    return _kwargs


def _parse_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> ContractVerification | Error | None:
    if response.status_code == 200:
        response_200 = ContractVerification.from_dict(response.json())

        return response_200

    if response.status_code == 404:
        response_404 = Error.from_dict(response.json())

        return response_404

    if response.status_code == 422:
        response_422 = Error.from_dict(response.json())

        return response_422

    if response.status_code == 500:
        response_500 = Error.from_dict(response.json())

        return response_500

    if response.status_code == 503:
        response_503 = Error.from_dict(response.json())

        return response_503

    if client.raise_on_unexpected_status:
        raise errors.UnexpectedStatus(response.status_code, response.content)
    else:
        return None


def _build_response(
    *, client: AuthenticatedClient | Client, response: httpx.Response
) -> Response[ContractVerification | Error]:
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
    body: ContractVerificationRequest,
) -> Response[ContractVerification | Error]:
    """Verify contract source against the on-chain Wasm hash

     Rebuilds the submitted source in an isolated sandbox and compares the
    resulting SHA-256 Wasm hash with the hash recorded on-chain for this
    contract. Supply exactly one source payload: `source.kind` is either
    `archive` (with `source.content_base64`) or `git` (with `source.url`
    and `source.commit`). `source.subdir` and `artifact` are optional
    overrides for multi-crate sources and for naming the built `.wasm`
    directly.

    A build that runs but does not match is a successful request: it
    returns 200 with `status: failed`, `matched: false`, and diagnostics
    explaining the likely cause. Use 422 for malformed input: an unknown
    `kind`, a missing payload, or a body over 16 MiB. A deployment without
    a build sandbox answers 503 rather than pretending to verify.

    Args:
        id (str):
        body (ContractVerificationRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContractVerification | Error]
    """

    kwargs = _get_kwargs(
        id=id,
        body=body,
    )

    response = client.get_httpx_client().request(
        **kwargs,
    )

    return _build_response(client=client, response=response)


def sync(
    id: str,
    *,
    client: AuthenticatedClient,
    body: ContractVerificationRequest,
) -> ContractVerification | Error | None:
    """Verify contract source against the on-chain Wasm hash

     Rebuilds the submitted source in an isolated sandbox and compares the
    resulting SHA-256 Wasm hash with the hash recorded on-chain for this
    contract. Supply exactly one source payload: `source.kind` is either
    `archive` (with `source.content_base64`) or `git` (with `source.url`
    and `source.commit`). `source.subdir` and `artifact` are optional
    overrides for multi-crate sources and for naming the built `.wasm`
    directly.

    A build that runs but does not match is a successful request: it
    returns 200 with `status: failed`, `matched: false`, and diagnostics
    explaining the likely cause. Use 422 for malformed input: an unknown
    `kind`, a missing payload, or a body over 16 MiB. A deployment without
    a build sandbox answers 503 rather than pretending to verify.

    Args:
        id (str):
        body (ContractVerificationRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContractVerification | Error
    """

    return sync_detailed(
        id=id,
        client=client,
        body=body,
    ).parsed


async def asyncio_detailed(
    id: str,
    *,
    client: AuthenticatedClient,
    body: ContractVerificationRequest,
) -> Response[ContractVerification | Error]:
    """Verify contract source against the on-chain Wasm hash

     Rebuilds the submitted source in an isolated sandbox and compares the
    resulting SHA-256 Wasm hash with the hash recorded on-chain for this
    contract. Supply exactly one source payload: `source.kind` is either
    `archive` (with `source.content_base64`) or `git` (with `source.url`
    and `source.commit`). `source.subdir` and `artifact` are optional
    overrides for multi-crate sources and for naming the built `.wasm`
    directly.

    A build that runs but does not match is a successful request: it
    returns 200 with `status: failed`, `matched: false`, and diagnostics
    explaining the likely cause. Use 422 for malformed input: an unknown
    `kind`, a missing payload, or a body over 16 MiB. A deployment without
    a build sandbox answers 503 rather than pretending to verify.

    Args:
        id (str):
        body (ContractVerificationRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        Response[ContractVerification | Error]
    """

    kwargs = _get_kwargs(
        id=id,
        body=body,
    )

    response = await client.get_async_httpx_client().request(**kwargs)

    return _build_response(client=client, response=response)


async def asyncio(
    id: str,
    *,
    client: AuthenticatedClient,
    body: ContractVerificationRequest,
) -> ContractVerification | Error | None:
    """Verify contract source against the on-chain Wasm hash

     Rebuilds the submitted source in an isolated sandbox and compares the
    resulting SHA-256 Wasm hash with the hash recorded on-chain for this
    contract. Supply exactly one source payload: `source.kind` is either
    `archive` (with `source.content_base64`) or `git` (with `source.url`
    and `source.commit`). `source.subdir` and `artifact` are optional
    overrides for multi-crate sources and for naming the built `.wasm`
    directly.

    A build that runs but does not match is a successful request: it
    returns 200 with `status: failed`, `matched: false`, and diagnostics
    explaining the likely cause. Use 422 for malformed input: an unknown
    `kind`, a missing payload, or a body over 16 MiB. A deployment without
    a build sandbox answers 503 rather than pretending to verify.

    Args:
        id (str):
        body (ContractVerificationRequest):

    Raises:
        errors.UnexpectedStatus: If the server returns an undocumented status code and Client.raise_on_unexpected_status is True.
        httpx.TimeoutException: If the request takes longer than Client.timeout.

    Returns:
        ContractVerification | Error
    """

    return (
        await asyncio_detailed(
            id=id,
            client=client,
            body=body,
        )
    ).parsed
