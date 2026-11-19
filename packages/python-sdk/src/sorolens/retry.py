"""Retry support shared by the synchronous and asynchronous clients.

``httpx`` has no built-in retry policy, so the SDK wraps the configured
transport with :class:`RetryTransport`. The same wrapper serves both transports
because it implements ``httpx.BaseTransport`` and ``httpx.AsyncBaseTransport``;
the generated client builds a sync and an async ``httpx`` client from the same
arguments, so this is what lets a single ``retry=`` option cover both.

The policy is deliberately conservative:

* only requests whose method is listed in ``retry_methods`` (``GET``/``HEAD`` by
  default, i.e. safe to replay) are retried,
* only the statuses in ``retry_statuses`` (429 and common 5xx) are retried,
* ``Retry-After`` is honoured for 429/503 (delta-seconds or HTTP-date) and
  capped at ``max_backoff``,
* exponential backoff with full jitter is used when no ``Retry-After`` is sent,
* async retries ``await asyncio.sleep``, so cancelling the task stops the retry
  loop immediately.
"""

from __future__ import annotations

import asyncio
import random
import time
from dataclasses import dataclass, field
from datetime import UTC, datetime
from email.utils import parsedate_to_datetime

import httpx

DEFAULT_RETRY_STATUSES = frozenset({429, 500, 502, 503, 504})


@dataclass(frozen=True)
class RetryPolicy:
    """How many times and how long to retry a retryable response."""

    max_retries: int = 2
    backoff_factor: float = 0.5
    max_backoff: float = 30.0
    retry_statuses: frozenset[int] = field(default=DEFAULT_RETRY_STATUSES)
    retry_methods: frozenset[str] = field(default=frozenset({"GET", "HEAD"}))

    def __post_init__(self) -> None:
        if self.max_retries < 0:
            raise ValueError("max_retries must be >= 0")
        if self.backoff_factor < 0:
            raise ValueError("backoff_factor must be >= 0")

    def delay(self, attempt: int, response: httpx.Response) -> float:
        """Seconds to wait before retry number ``attempt`` (zero-based)."""
        retry_after = parse_retry_after(response.headers.get("Retry-After"))
        if retry_after is not None:
            return min(retry_after, self.max_backoff)
        base = self.backoff_factor * (2**attempt)
        # Full jitter: anywhere in [base/2, base], never above max_backoff.
        jittered = base / 2 + random.random() * (base / 2)
        return min(jittered, self.max_backoff)


DEFAULT_RETRY_POLICY = RetryPolicy()


def parse_retry_after(value: str | None, *, now: datetime | None = None) -> float | None:
    """Parse an HTTP ``Retry-After`` value into seconds, or ``None``."""
    if not value:
        return None
    value = value.strip()
    try:
        seconds = int(value)
    except ValueError:
        try:
            when = parsedate_to_datetime(value)
        except (TypeError, ValueError):
            return None
        if when.tzinfo is None:
            when = when.replace(tzinfo=UTC)
        current = now or datetime.now(UTC)
        return max((when - current).total_seconds(), 0.0)
    return float(max(seconds, 0))


class RetryTransport(httpx.BaseTransport, httpx.AsyncBaseTransport):
    """A transport that retries safe requests on 429/5xx.

    ``transport`` may be any httpx transport (sync, async, or a ``MockTransport``
    that implements both). When omitted, a fresh ``httpcore`` transport is
    created lazily for each direction.
    """

    def __init__(
        self,
        transport: httpx.BaseTransport | httpx.AsyncBaseTransport | None = None,
        policy: RetryPolicy | None = None,
        *,
        sleep: object | None = None,
    ) -> None:
        self._transport = transport
        self._sync_transport: httpx.BaseTransport | None = None
        self._async_transport: httpx.AsyncBaseTransport | None = None
        self.policy = policy or DEFAULT_RETRY_POLICY
        self._sleep = sleep

    # -- sync ---------------------------------------------------------------
    def _sync(self) -> httpx.BaseTransport:
        if self._sync_transport is None:
            inner = self._transport
            if inner is None:
                inner = httpx.HTTPTransport()
            self._sync_transport = inner  # type: ignore[assignment]
        return self._sync_transport

    def handle_request(self, request: httpx.Request) -> httpx.Response:
        sleep = self._sleep if callable(self._sleep) else time.sleep
        attempt = 0
        while True:
            response = self._sync().handle_request(request)
            if not self._should_retry(request, response, attempt):
                return response
            delay = self.policy.delay(attempt, response)
            response.close()
            attempt += 1
            sleep(delay)

    # -- async --------------------------------------------------------------
    def _async(self) -> httpx.AsyncBaseTransport:
        if self._async_transport is None:
            inner = self._transport
            if inner is None:
                inner = httpx.AsyncHTTPTransport()
            self._async_transport = inner  # type: ignore[assignment]
        return self._async_transport

    async def handle_async_request(self, request: httpx.Request) -> httpx.Response:
        attempt = 0
        while True:
            response = await self._async().handle_async_request(request)
            if not self._should_retry(request, response, attempt):
                return response
            delay = self.policy.delay(attempt, response)
            await response.aclose()
            attempt += 1
            await asyncio.sleep(delay)

    # -- shared -------------------------------------------------------------
    def _should_retry(self, request: httpx.Request, response: httpx.Response, attempt: int) -> bool:
        if attempt >= self.policy.max_retries:
            return False
        if request.method.upper() not in self.policy.retry_methods:
            return False
        return response.status_code in self.policy.retry_statuses
