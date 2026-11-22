"""Error handling: every non-2xx response becomes a ``SorolensError`` subclass.

Run against a live server (expect a 404 for the missing contract)::

    SOROLENS_API_URL=https://api.sorolens.xyz python examples/errors.py

Without ``SOROLENS_API_URL`` the example replays canned 404/422/429 responses
through an ``httpx.MockTransport`` and asserts the mapping offline.
"""

from __future__ import annotations

import os

import httpx

from sorolens import (
    APIError,
    Client,
    NotFoundError,
    RateLimitError,
    ValidationError,
)

MISSING_ID = "CMISSINGCONTRACTIDENTIFIER00000000000000000000000000000000"
INVALID_ID = "CINVALIDCONTRACTIDENTIFIER000000000000000000000000000000000"

ERROR_404 = {
    "error": {
        "code": "NOT_FOUND",
        "message": "contract not found",
        "request_id": "0f8b0f0e-6a1f-4f0f-9f4e-2b1c9f0e6a1f",
    }
}
ERROR_422 = {
    "error": {
        "code": "INVALID_INPUT",
        "message": "invalid contract id",
        "request_id": "7a2f1a8e-1e0f-4f5b-9b7e-3d2c1b0a9f8e",
    }
}
ERROR_429 = {
    "error": {
        "code": "RATE_LIMITED",
        "message": "too many requests",
        "request_id": "2b3c4d5e-6f70-4182-93a4-b5c6d7e8f901",
    }
}


def offline_handler(request: httpx.Request) -> httpx.Response:
    path = request.url.path
    if path.endswith(MISSING_ID):
        return httpx.Response(404, json=ERROR_404)
    if path.endswith(INVALID_ID):
        return httpx.Response(422, json=ERROR_422)
    return httpx.Response(429, json=ERROR_429)


def build_client() -> Client:
    base_url = os.environ.get("SOROLENS_API_URL")
    if base_url:
        return Client(base_url=base_url, api_key=os.environ.get("SOROLENS_API_KEY"))
    return Client(
        base_url="https://offline.invalid",
        httpx_args={"transport": httpx.MockTransport(offline_handler)},
    )


def report(exc: Exception) -> None:
    status = getattr(exc, "status_code", None)
    print(f"  {type(exc).__name__}: {exc} (status={status})")


def main() -> int:
    offline = not os.environ.get("SOROLENS_API_URL")
    with build_client() as client:
        print("missing contract:")
        try:
            client.contracts.get(MISSING_ID)
        except NotFoundError as exc:
            report(exc)
            if offline:
                assert exc.status_code == 404
                assert "contract not found" in exc.message

        print("invalid contract id:")
        try:
            client.contracts.get(INVALID_ID)
        except ValidationError as exc:
            report(exc)
        except APIError as exc:
            # A live server may answer 404 for an id it does not know yet.
            report(exc)

        print("rate limit:")
        try:
            client.contracts.list(limit=10)
        except RateLimitError as exc:
            report(exc)
            if offline:
                assert exc.status_code == 429
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
