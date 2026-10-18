"""SoroLens Python SDK.

Provides auto-pagination iterators for events and invocations,
both synchronous and asynchronous.
"""

from __future__ import annotations

from .client import AsyncSoroLensClient, SoroLensClient
from .pagination import (
    AsyncInvocationIterator,
    AsyncEventIterator,
    EventIterator,
    InvocationIterator, iter_events, iter_invocations, iter_events_async, iter_invocations_async,
)
from .models import Event, Invocation, Page

__all__ = [
    "SoroLensClient",
    "AsyncSoroLensClient",
    "Event",
    "Invocation",
    "Page",
    "EventIterator",
    "InvocationIterator",
    "AsyncEventIterator",
    "AsyncInvocationIterator",
    "iter_events",
    "iter_invocations",
    "iter_events_async",
    "iter_invocations_async",
]

__version__ = "0.1.0"
