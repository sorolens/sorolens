import asyncio

import pytest

from sorolens import Client, AsyncClient


class FakeTransport:
    def __init__(self, pages):
        self.pages = pages
        self.calls = []

    def fetch(self, cursor):
        self.calls.append(cursor)
        return self.pages[cursor]


def test_iter_events_multi_page():
    transport = FakeTransport({
        None: {"items": [1, 2], "cursor": "a"},
        "a": {"items": [3], "cursor": "b"},
        "b": {"items": [], "cursor": None},
    })
    client = Client(transport=fake_fetch(transport))
    assert list(client.iter_events()) == [1, 2, 3]
    assert transport.calls == [None, "a", "b"]


def test_iter_invocations_multi_page():
    transport = FakeTransport({
        None: {"items": ["call-1"], "cursor": "x"},
        "x": {"items": ["call-2", "call-3"], "cursor": None},
    })
    client = Client(transport=fake_fetch(transport))
    assert list(client.iter_invocations()) == ["call-1", "call-2", "call-3"]


@pytest.mark.asyncio
async def test_async_iter_events_multi_page():
    transport = FakeTransport({
        None: {"items": [1], "cursor": "a"},
        "a": {"items": [2, 3], "cursor": None},
    })
    client = AsyncClient(transport=fake_async_fetch(transport))
    out = []
    async for item in client.iter_events():
        out.append(item)
    assert out == [1, 2, 3]


def fake_fetch(transport):
    def _fetch(cursor):
        return transport.fetch(cursor)
    return _fetch


def fake_async_fetch(transport):
    async def _fetch(cursor):
        return transport.fetch(cursor)
    return _fetch
