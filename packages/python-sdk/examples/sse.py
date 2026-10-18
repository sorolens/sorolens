"""Consume the Server-Sent Events endpoint ``GET /api/v1/stream/events``.

The high-level ``client.events.stream(contract_id)`` helper short-polls
``/api/v1/contracts/{id}/stream``; this example reads the real SSE endpoint
with the same httpx transport the SDK is built on, so every ``data:`` line is
handled as it arrives.

Run against a live server::

    SOROLENS_API_URL=https://api.sorolens.xyz python examples/sse.py

Without ``SOROLENS_API_URL`` a canned SSE transcript is replayed offline, which
is what CI runs.
"""

from __future__ import annotations

import json
import os
from collections.abc import Iterator

import httpx

DEFAULT_BASE_URL = "https://api.sorolens.xyz"

OFFLINE_TRANSCRIPT = (
    "event: contract.event\n"
    'data: {"type":"contract.event","id":"0","contract_id":"CDEMO"}\n'
    "\n"
    "event: contract.event\n"
    'data: {"type":"contract.event","id":"1","contract_id":"CDEMO"}\n'
    "\n"
    ": keep-alive\n"
    "event: contract.event\n"
    'data: {"type":"contract.event","id":"2","contract_id":"CDEMO"}\n'
    "\n"
)


def offline_handler(request: httpx.Request) -> httpx.Response:
    return httpx.Response(
        200,
        content=OFFLINE_TRANSCRIPT.encode("utf-8"),
        headers={"content-type": "text/event-stream"},
    )


def build_http() -> httpx.Client:
    base_url = os.environ.get("SOROLENS_API_URL")
    if base_url:
        headers = {}
        api_key = os.environ.get("SOROLENS_API_KEY")
        if api_key:
            headers["Authorization"] = f"Bearer {api_key}"
        return httpx.Client(base_url=base_url, headers=headers, timeout=None)
    return httpx.Client(
        base_url="https://offline.invalid",
        transport=httpx.MockTransport(offline_handler),
    )


def iter_events(http: httpx.Client) -> Iterator[dict]:
    """Yield the JSON payload of every ``data:`` line in the stream."""
    with http.stream("GET", "/api/v1/stream/events") as response:
        response.raise_for_status()
        for line in response.iter_lines():
            if not line.startswith("data:"):
                continue
            payload = line[len("data:") :].strip()
            if payload:
                yield json.loads(payload)


def main() -> int:
    offline = not os.environ.get("SOROLENS_API_URL")
    with build_http() as http:
        events = list(iter_events(http))

    print(f"received {len(events)} events")
    for event in events:
        print(f"  {event.get('type')} {event.get('id')}")

    if offline:
        assert len(events) == 3, events
        assert all(event["contract_id"] == "CDEMO" for event in events)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
