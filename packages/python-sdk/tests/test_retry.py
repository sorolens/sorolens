"""Retry behaviour shared by the sync and async clients."""

from __future__ import annotations

import asyncio

import httpx
import pytest

from conftest import BASE_URL, load
from sorolens import AsyncClient, Client, RetryPolicy, RetryTransport, exceptions

STATS = load("global_stats.json")


def flaky_transport(failures: int, status: int = 429, headers: dict[str, str] | None = None):
    """Return a transport that fails ``failures`` times, then succeeds."""
    calls = {"n": 0}

    def handler(request: httpx.Request) -> httpx.Response:
        calls["n"] += 1
        if calls["n"] <= failures:
            return httpx.Response(
                status,
                headers=headers or {},
                json={"error": {"code": "RETRY", "message": "retry me"}},
            )
        return httpx.Response(200, json=STATS)

    return httpx.MockTransport(handler), calls


def test_sync_retries_and_honours_retry_after():
    transport, calls = flaky_transport(2, headers={"Retry-After": "0"})
    with Client(base_url=BASE_URL, httpx_args={"transport": transport}) as client:
        stats = client.stats.global_stats()
    assert stats.tracked_contracts == 42
    assert calls["n"] == 3


def test_sync_gives_up_after_max_retries():
    transport, calls = flaky_transport(99, headers={"Retry-After": "0"})
    with Client(base_url=BASE_URL, httpx_args={"transport": transport}) as client:
        with pytest.raises(exceptions.RateLimitError):
            client.stats.global_stats()
    # one initial attempt plus the two default retries
    assert calls["n"] == 3


def test_retry_can_be_disabled():
    transport, calls = flaky_transport(1)
    with Client(base_url=BASE_URL, retry=False, httpx_args={"transport": transport}) as client:
        with pytest.raises(exceptions.RateLimitError):
            client.stats.global_stats()
    assert calls["n"] == 1


def test_post_is_not_retried():
    transport, calls = flaky_transport(1, status=500)
    with Client(base_url=BASE_URL, httpx_args={"transport": transport}) as client:
        with pytest.raises(exceptions.APIError):
            client.contracts.register("C1", "testnet")
    assert calls["n"] == 1


def test_retry_transport_uses_exponential_backoff():
    request = httpx.Request("GET", "https://api.test/api/v1/stats/global")
    response = httpx.Response(503, request=request)
    policy = RetryPolicy(max_retries=3, backoff_factor=1.0, max_backoff=10.0)
    delays = [policy.delay(attempt, response) for attempt in range(3)]
    assert all(delay <= 10.0 for delay in delays)
    assert delays[0] <= 1.0 < delays[2]


def test_retry_after_http_date_is_parsed():
    request = httpx.Request("GET", "https://api.test/api/v1/stats/global")
    response = httpx.Response(
        429, headers={"Retry-After": "Wed, 21 Oct 2015 07:28:00 GMT"}, request=request
    )
    policy = RetryPolicy(max_backoff=30.0)
    # A date in the past is already elapsed, so retrying immediately is right.
    assert policy.delay(0, response) == 0.0


async def test_async_retries_and_honours_retry_after():
    transport, calls = flaky_transport(2, headers={"Retry-After": "0"})
    async with AsyncClient(base_url=BASE_URL, httpx_args={"transport": transport}) as client:
        stats = await client.stats.global_stats()
    assert stats.tracked_contracts == 42
    assert calls["n"] == 3


async def test_async_cancellation_stops_retry_loop():
    transport, calls = flaky_transport(99)
    policy = RetryPolicy(max_retries=10, backoff_factor=30.0, max_backoff=30.0)
    async with AsyncClient(
        base_url=BASE_URL, retry=policy, httpx_args={"transport": transport}
    ) as client:
        task = asyncio.create_task(client.stats.global_stats())
        await asyncio.sleep(0.05)
        task.cancel()
        with pytest.raises(asyncio.CancelledError):
            await asyncio.wait_for(task, timeout=1.0)
    # Cancelled during the first backoff, before a second request.
    assert calls["n"] == 1


async def test_retry_transport_async_forwards_success():
    request = httpx.Request("GET", "https://api.test/health")

    class Transport(httpx.AsyncBaseTransport):
        async def handle_async_request(self, request: httpx.Request) -> httpx.Response:
            return httpx.Response(200, json={"status": "ok"})

    transport = RetryTransport(Transport(), RetryPolicy())
    response = await transport.handle_async_request(request)
    assert response.status_code == 200
