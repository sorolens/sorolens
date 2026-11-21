"""Auto-pagination over events and invocations (issue #770).

Every test serves several cursor pages through ``httpx.MockTransport`` and
records each request, so the suite checks what was fetched and when, not only
what came out.
"""

from __future__ import annotations

import copy

import httpx
import pytest

from conftest import BASE_URL, CONTRACT_ID, load
from sorolens import AsyncClient, Client, PaginationError, SorolensError

EVENT = load("events_page.json")["events"][0]
INVOCATION = load("invocations_page.json")["invocations"][0]


def event(page: int, n: int) -> dict:
    item = copy.deepcopy(EVENT)
    item["id"] = f"evt-{page}-{n}"
    return item


def invocation(page: int, n: int) -> dict:
    item = copy.deepcopy(INVOCATION)
    item["tx_hash"] = f"tx-{page}-{n}"
    return item


class PagedApi:
    """Serves ``pages`` pages of two items each, chained by ``p<n>`` cursors."""

    def __init__(
        self,
        key: str,
        make,
        *,
        pages: int = 3,
        fail_on: int | None = None,
        cursor_for=None,
    ) -> None:
        self.key = key
        self.make = make
        self.pages = pages
        self.fail_on = fail_on
        self.cursor_for = cursor_for or (lambda page: f"p{page + 1}")
        self.requests: list[httpx.Request] = []

    def page_index(self, request: httpx.Request) -> int:
        cursor = request.url.params.get("cursor")
        return int(cursor[1:]) if cursor else 0

    def __call__(self, request: httpx.Request) -> httpx.Response:
        self.requests.append(request)
        index = self.page_index(request)
        if index == self.fail_on:
            return httpx.Response(500, json={"error": {"code": "INTERNAL", "message": "boom"}})
        last = index == self.pages - 1
        return httpx.Response(
            200,
            json={
                self.key: [self.make(index, 0), self.make(index, 1)],
                "next_cursor": "" if last else self.cursor_for(index),
            },
        )

    def expected(self, field: str, pages: int | None = None) -> list[str]:
        return [self.make(page, n)[field] for page in range(pages or self.pages) for n in range(2)]

    def cursors(self) -> list[str | None]:
        return [request.url.params.get("cursor") for request in self.requests]


def sync_client(api: PagedApi) -> Client:
    return Client(
        base_url=BASE_URL, retry=False, httpx_args={"transport": httpx.MockTransport(api)}
    )


def async_client(api: PagedApi) -> AsyncClient:
    return AsyncClient(
        base_url=BASE_URL, retry=False, httpx_args={"transport": httpx.MockTransport(api)}
    )


# --------------------------------------------------------------------------- #
# Synchronous


def test_iter_events_walks_every_page_and_stops_on_the_last_cursor():
    api = PagedApi("events", event)
    with sync_client(api) as client:
        ids = [item.id for item in client.iter_events(CONTRACT_ID, limit=2)]
    assert ids == api.expected("id")
    assert api.cursors() == [None, "p1", "p2"]
    assert all(request.url.params.get("limit") == "2" for request in api.requests)


def test_iter_invocations_walks_every_page_with_filters():
    api = PagedApi("invocations", invocation)
    with sync_client(api) as client:
        hashes = [
            item.tx_hash
            for item in client.iter_invocations(CONTRACT_ID, status="FAILED", fn="transfer")
        ]
    assert hashes == api.expected("tx_hash")
    assert len(api.requests) == 3
    assert {request.url.params.get("status") for request in api.requests} == {"FAILED"}
    assert {request.url.params.get("function_name") for request in api.requests} == {"transfer"}


def test_iterators_fetch_pages_lazily():
    api = PagedApi("events", event)
    with sync_client(api) as client:
        events = client.iter_events(CONTRACT_ID)
        assert api.requests == []
        assert next(events).id == "evt-0-0"
        assert next(events).id == "evt-0-1"
        assert len(api.requests) == 1
        assert next(events).id == "evt-1-0"
        assert len(api.requests) == 2
        events.close()
    assert len(api.requests) == 2


@pytest.mark.parametrize(
    ("key", "make", "field", "method"),
    [
        ("events", event, "id", "iter_events"),
        ("invocations", invocation, "tx_hash", "iter_invocations"),
    ],
)
def test_error_mid_stream_is_raised_after_items_already_received(key, make, field, method):
    api = PagedApi(key, make, fail_on=1)
    received: list[str] = []
    with sync_client(api) as client:
        with pytest.raises(SorolensError):
            for item in getattr(client, method)(CONTRACT_ID):
                received.append(getattr(item, field))
    assert received == api.expected(field, pages=1)
    assert api.cursors() == [None, "p1"]


def test_page_size_is_clamped_to_the_server_maximum():
    api = PagedApi("events", event, pages=1)
    with sync_client(api) as client:
        list(client.iter_events(CONTRACT_ID, limit=10_000))
    assert api.requests[0].url.params.get("limit") == "200"


def test_page_size_below_one_is_rejected_before_any_request():
    api = PagedApi("invocations", invocation)
    with sync_client(api) as client:
        with pytest.raises(ValueError):
            next(client.iter_invocations(CONTRACT_ID, limit=0))
    assert api.requests == []


def test_repeated_cursor_stops_the_walk_instead_of_looping():
    # Every page points back at "p1", so the second page would repeat forever.
    api = PagedApi("events", event, pages=5, cursor_for=lambda page: "p1")
    received: list[str] = []
    with sync_client(api) as client:
        with pytest.raises(PaginationError):
            for item in client.iter_events(CONTRACT_ID):
                received.append(item.id)
    assert received == api.expected("id", pages=2)
    assert len(api.requests) == 2


def test_resource_iterators_share_the_same_behaviour():
    api = PagedApi("events", event)
    with sync_client(api) as client:
        assert [e.id for e in client.events.iter(CONTRACT_ID)] == api.expected("id")


# --------------------------------------------------------------------------- #
# Asynchronous


async def test_async_iter_events_walks_every_page():
    api = PagedApi("events", event)
    async with async_client(api) as client:
        ids = [item.id async for item in client.iter_events(CONTRACT_ID, limit=2)]
    assert ids == api.expected("id")
    assert api.cursors() == [None, "p1", "p2"]


async def test_async_iter_invocations_walks_every_page():
    api = PagedApi("invocations", invocation)
    async with async_client(api) as client:
        hashes = [item.tx_hash async for item in client.iter_invocations(CONTRACT_ID)]
    assert hashes == api.expected("tx_hash")
    assert len(api.requests) == 3


async def test_async_iterators_fetch_pages_lazily():
    api = PagedApi("invocations", invocation)
    async with async_client(api) as client:
        invocations = client.iter_invocations(CONTRACT_ID)
        assert api.requests == []
        assert (await anext(invocations)).tx_hash == "tx-0-0"
        assert (await anext(invocations)).tx_hash == "tx-0-1"
        assert len(api.requests) == 1
        await invocations.aclose()
    assert len(api.requests) == 1


async def test_async_error_mid_stream_is_raised_after_items_already_received():
    api = PagedApi("events", event, fail_on=2)
    received: list[str] = []
    async with async_client(api) as client:
        with pytest.raises(SorolensError):
            async for item in client.iter_events(CONTRACT_ID):
                received.append(item.id)
    assert received == api.expected("id", pages=2)


async def test_async_repeated_cursor_raises_pagination_error():
    api = PagedApi("invocations", invocation, pages=5, cursor_for=lambda page: "p1")
    async with async_client(api) as client:
        with pytest.raises(PaginationError):
            async for _ in client.iter_invocations(CONTRACT_ID, limit=500):
                pass
    assert api.requests[0].url.params.get("limit") == "200"
