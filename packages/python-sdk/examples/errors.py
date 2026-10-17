"""Handle API errors with the SDK's exception hierarchy.

Every non-2xx response raises a subclass of :class:`sorolens.APIError` carrying
``status_code`` and ``message``, so you can branch on the kind of failure instead
of parsing status codes. Network failures (DNS, connection refused, timeouts)
are not wrapped: they surface as :class:`httpx.HTTPError`.

Against a deployment::

    SOROLENS_BASE_URL=http://localhost:8080 SOROLENS_API_KEY=sl_dev_... \\
        python examples/errors.py

Offline self-check (what CI runs)::

    python examples/errors.py --self-test
"""

from __future__ import annotations

import argparse
import os

import httpx

import sorolens

# Well-formed but unregistered, so the API answers 404 rather than a format error.
MISSING_CONTRACT = "C" + "A" * 55


def probe(client: sorolens.Client) -> list[str]:
    """Make two calls that can fail and report how each one ended."""
    outcomes: list[str] = []

    try:
        client.contracts.get(MISSING_CONTRACT)
        outcomes.append("get_contract: ok")
    except sorolens.NotFoundError as exc:
        # 404: the resource does not exist. Usually safe to treat as "absent".
        outcomes.append(f"get_contract: HTTP {exc.status_code} not found - {exc.message}")

    try:
        client.stats.global_stats()
        outcomes.append("get_global_stats: ok")
    except sorolens.RateLimitError as exc:
        # 429: back off and retry later; retrying immediately only makes it worse.
        outcomes.append(f"get_global_stats: HTTP {exc.status_code} rate limited - {exc.message}")
    except sorolens.AuthenticationError as exc:
        # 401: the API key is missing or invalid. Retrying will not help.
        outcomes.append(f"get_global_stats: HTTP {exc.status_code} unauthenticated")
    except sorolens.APIError as exc:
        # Any other non-2xx status (403, 422, 5xx...) lands here.
        outcomes.append(f"get_global_stats: HTTP {exc.status_code} - {exc.message}")
    except httpx.HTTPError as exc:
        # No HTTP response at all: DNS failure, connection refused, timeout.
        outcomes.append(f"get_global_stats: network error - {exc}")

    return outcomes


def _mock_api(request: httpx.Request) -> httpx.Response:
    if request.url.path == f"/api/v1/contracts/{MISSING_CONTRACT}":
        body = {"error": {"code": "NOT_FOUND", "message": "contract not found"}}
        return httpx.Response(404, json=body)
    body = {"error": {"code": "RATE_LIMITED", "message": "too many requests"}}
    return httpx.Response(429, json=body)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--self-test", action="store_true", help="run against a built-in mock")
    args = parser.parse_args(argv)

    if args.self_test:
        client = sorolens.Client(
            base_url="https://api.test", httpx_args={"transport": httpx.MockTransport(_mock_api)}
        )
        with client:
            outcomes = probe(client)
        print("\n".join(outcomes))
        if "HTTP 404 not found" not in outcomes[0] or "HTTP 429 rate limited" not in outcomes[1]:
            print("self-test: errors were not mapped to the expected exceptions")
            return 1
        return 0

    base_url = os.environ.get("SOROLENS_BASE_URL", sorolens.DEFAULT_BASE_URL)
    with sorolens.Client(base_url=base_url, api_key=os.environ.get("SOROLENS_API_KEY")) as client:
        print("\n".join(probe(client)))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
