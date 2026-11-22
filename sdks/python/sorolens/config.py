"""Configuration for the Sorolens Python SDK.

This module defines the configuration object used by the HTTP client,
including retry behavior for transient errors (429 and 5xx).

Defaults (documented in the README):
    - max_retries: 3
    - base_delay: 0.5 seconds
    - max_delay: 30.0 seconds
    - backoff_factor: 2.0
    - jitter: 0.25
    - retry_statuses: {429} | 5xx
    - respect_retry_after: True
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import FrozenSet, Optional


DEFAULT_RETRY_STATUSES: FrozenSet[int] = frozenset([429])


@dataclass(frozen=True)
class RetryConfig:
    """Configuration for retrying failed requests.

    Attributes:
        max_retries: Maximum number of retry attempts after the initial request.
            Set to 0 to disable retries.
        base_delay: Initial backoff delay in seconds.
        max_delay: Upper bound on any single backoff delay in seconds.
        backoff_factor: Multiplier applied to the delay after each attempt.
        jitter: Fraction of random jitter applied to each delay (0.0 - 1.0).
        retry_statuses: Explicit status codes to retry. 5xx responses are
            always retried regardless of this set.
        respect_retry_after: Honor the `Retry-After` response header when present.
        retry_on_connection_errors: Retry on transport-level connection errors.
    """

    max_retries: int = 3
    base_delay: float = 0.5
    max_delay: float = 30.0
    backoff_factor: float = 2.0
    jitter: float = 0.25
    retry_statuses: FrozenSet[int] = field(default=factory=lambda: DEFAULT_RETRY_STATUSES)
    respect_retry_after: bool = True
    retry_on_connection_errors: bool = True

    def __post_init__(self) -> None:
        if self.max_retries < 0:
            raise ValueError("max_retries must be >= 0")
        if self.base_delay < 0:
            raise ValueError("base_delay must be >= 0")
        if self.max_delay < 0 or self.max_delay < self.base_delay:
            raise ValueError("max_delay must be >= base_delay")
        if self.backoff_factor < 1.0:
            raise ValueError("backoff_factor must be >= 1.0")
        if not 0.0 <= self.jitter <= 1.0:
            raise ValueError("jitter must be between 0.0 and 1.0")

    def should_retry_status(self, status_code: int) -> bool:
        """Return True if the given status code is retryable."""
        if 500 <= status_code <= 599:
            return True
        return status_code in self.retry_statuses


@dataclass(frozen=True)
class ClientConfig:
    """Top-level configuration for the Sorolens client."""

    base_url: str = "https://api.sorolens.xyz"
    api_key: Optional[str] = None
    timeout: float = 30.0
    retry: RetryConfig = field(default_factory=RetryConfig)
