"""Consume the ``/api/v1/stream/events`` Server-Sent Events stream.

The SDK's ``client.events.stream()`` is a short-poll convenience call that returns
the 20 most recent events of one contract as JSON. The live, long-lived stream is
plain SSE, so it is read here with ``httpx`` (already an SDK dependency).

Against a deployment (stop with Ctrl+C, or add ``--max-events 5``)::

    SOROLENS_BASE_URL=http://localhost:8080 SOROLENS_API_KEY=sl_dev_... \\
        python examples/sse.py

Narrow it to one contract with ``SOROLENS_CONTRACT_ID``.

Offline self-check (what CI runs)::

    python examples/sse.py --self-test
"""

from __future__ import annotations

import argparse
import json
import os

import httpx

import sorolens

EVENTS = (
    b'data: {"id":"evt_1","contract_id":"CAAA","type":"transfer","ledger":101}\n\n'
    b": keep-alive comment lines and other fields are skipped\n\n"
    b'data: {"id":"evt_2","contract_id":"CAAA","type":"mint","ledger":102}\n\n'
)


def consume(
    http: httpx.Client,
    base_url: str,
    *,
    api_key: str | None = None,
    contract_id: str | None = None,
    max_events: int | None = None,
) -> int:
    """Print each event as it arrives and return how many were read."""
    headers = {"Accept": "text/event-stream"}
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"
    params = {"contract_id": contract_id} if contract_id else None

    seen = 0
    url = f"{base_url.rstrip('/')}/api/v1/stream/events"
    with http.stream("GET", url, headers=headers, params=params) as response:
        response.raise_for_status()
        for line in response.iter_lines():
            if not line.startswith("data:"):
                continue  # comments (": ..."), "event:" / "id:" fields, blank separators
            try:
                event = json.loads(line.removeprefix("data:").strip())
            except json.JSONDecodeError:
                continue  # skip a malformed frame instead of killing the stream
            print(event["id"], event["contract_id"], event["type"])
            seen += 1
            if max_events is not None and seen >= max_events:
                break
    return seen


def _mock_api(request: httpx.Request) -> httpx.Response:
    return httpx.Response(200, headers={"content-type": "text/event-stream"}, content=EVENTS)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--self-test", action="store_true", help="run against a built-in mock")
    parser.add_argument("--max-events", type=int, help="stop after this many events")
    args = parser.parse_args(argv)

    if args.self_test:
        with httpx.Client(transport=httpx.MockTransport(_mock_api)) as http:
            seen = consume(http, "https://api.test")
        if seen != 2:
            print(f"self-test expected 2 events, got {seen}")
            return 1
        print(f"self-test: read {seen} events")
        return 0

    base_url = os.environ.get("SOROLENS_BASE_URL", sorolens.DEFAULT_BASE_URL)
    # No read timeout: an idle stream is normal, not a failure.
    with httpx.Client(timeout=httpx.Timeout(10.0, read=None)) as http:
        consume(
            http,
            base_url,
            api_key=os.environ.get("SOROLENS_API_KEY"),
            contract_id=os.environ.get("SOROLENS_CONTRACT_ID"),
            max_events=args.max_events,
        )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
