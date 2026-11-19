"""Pagination iterators exposed by the sync and async facades."""

from __future__ import annotations

import httpx

from conftest import BASE_URL, CONTRACT_ID, load
from sorolens import AsyncClient, Client

CONTRACTS = load("contracts_page.json")
EVENTS = load("events_page.json")


def paged_handler(page: dict, key: str):
    """Serve two pages: the fixture, then one item with a fresh id."""

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.params.get("cursor") == "page-2":
            item = dict(page[key][0])
            item["id"] = "second-page"
            return httpx.Response(200, json={key: [item], "next_cursor": ""})
        return httpx.Response(200, json={**page, "next_cursor": "page-2"})

    return httpx.MockTransport(handler)


def test_contract_iterator_walks_every_page():
    client = Client(
        base_url=BASE_URL, httpx_args={"transport": paged_handler(CONTRACTS, "contracts")}
    )
    ids = [contract.id for contract in client.contracts.iter(limit=2)]
    assert ids == [c["id"] for c in CONTRACTS["contracts"]] + ["second-page"]


def test_event_iterator_walks_every_page():
    client = Client(base_url=BASE_URL, httpx_args={"transport": paged_handler(EVENTS, "events")})
    ids = [event.id for event in client.events.iter(CONTRACT_ID)]
    assert ids == [e["id"] for e in EVENTS["events"]] + ["second-page"]


async def test_async_contract_iterator_walks_every_page():
    async with AsyncClient(
        base_url=BASE_URL, httpx_args={"transport": paged_handler(CONTRACTS, "contracts")}
    ) as client:
        ids = [contract.id async for contract in client.contracts.iter()]
    assert ids == [c["id"] for c in CONTRACTS["contracts"]] + ["second-page"]


async def test_async_event_iterator_walks_every_page():
    async with AsyncClient(
        base_url=BASE_URL, httpx_args={"transport": paged_handler(EVENTS, "events")}
    ) as client:
        ids = [event.id async for event in client.events.iter(CONTRACT_ID)]
    assert ids == [e["id"] for e in EVENTS["events"]] + ["second-page"]
