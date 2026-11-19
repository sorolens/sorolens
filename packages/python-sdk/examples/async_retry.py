"""Async client: retries, cancellation and pagination iterators.

Run against a live server::

    SOROLENS_API_URL=https://api.sorolens.xyz python examples/async_retry.py

Without ``SOROLENS_API_URL`` a mock transport answers with one ``429`` (carrying
``Retry-After: 0``) followed by two event pages, so the example exercises the
retry loop and ``async for`` pagination offline. CI runs it that way.
"""

from __future__ import annotations

import asyncio
import os

import httpx

from sorolens import AsyncClient, RetryPolicy

CONTRACT_ID = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
PAGE_ONE = {
    "events": [
        {
            "id": "evt_01",
            "contract_id": CONTRACT_ID,
            "network": "testnet",
            "ledger": 51240,
            "ledger_closed_at": "2026-01-15T12:05:00Z",
            "tx_hash": "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
            "type": "contract",
            "topic_xdr": ["AAAAAQ=="],
            "value_xdr": "AAAAAg==",
            "topic_decoded": ["counter"],
            "value_decoded": {"value": 7},
            "in_successful_call": True,
        }
    ],
    "next_cursor": "page-2",
}
PAGE_TWO = {**PAGE_ONE, "events": [{**PAGE_ONE["events"][0], "id": "evt_02"}], "next_cursor": ""}


def offline_handler(request: httpx.Request) -> httpx.Response:
    if request.url.params.get("cursor") == "page-2":
        return httpx.Response(200, json=PAGE_TWO)
    return httpx.Response(200, json=PAGE_ONE)


class RetryOnceTransport(httpx.AsyncBaseTransport):
    """Answer 429 once, then behave like the mock transport."""

    def __init__(self) -> None:
        self.failed = False

    async def handle_async_request(self, request: httpx.Request) -> httpx.Response:
        if not self.failed:
            self.failed = True
            return httpx.Response(429, headers={"Retry-After": "0"}, json={"error": "slow down"})
        return offline_handler(request)


def build_client() -> AsyncClient:
    base_url = os.environ.get("SOROLENS_API_URL")
    if base_url:
        return AsyncClient(
            base_url=base_url,
            api_key=os.environ.get("SOROLENS_API_KEY"),
            retry=RetryPolicy(max_retries=3),
        )
    return AsyncClient(
        base_url="https://offline.invalid",
        httpx_args={"transport": RetryOnceTransport()},
    )


async def main() -> int:
    offline = not os.environ.get("SOROLENS_API_URL")
    async with build_client() as client:
        ids = [event.id async for event in client.events.iter(CONTRACT_ID, limit=1)]

    print(f"received {len(ids)} events: {', '.join(ids)}")
    if offline:
        assert ids == ["evt_01", "evt_02"], ids
    return 0


if __name__ == "__main__":
    raise SystemExit(asyncio.run(main()))
