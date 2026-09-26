"""Asynchronous client tests against recorded API fixtures."""

from __future__ import annotations

import pytest

from conftest import CONTRACT_ID, make_async_client
from sorolens import exceptions


@pytest.fixture
async def client():
    async_client = make_async_client()
    yield async_client
    await async_client.close()


async def test_async_global_stats(client):
    stats = await client.stats.global_stats()
    assert stats.tracked_contracts == 42


async def test_async_list_contracts(client):
    page = await client.contracts.list(limit=2)
    assert len(page.contracts) == 2
    assert page.contracts[0].network == "testnet"


async def test_async_get_missing_contract_raises_not_found(client):
    with pytest.raises(exceptions.NotFoundError):
        await client.contracts.get("missing")


async def test_async_list_events(client):
    page = await client.events.list(CONTRACT_ID)
    assert page.events[0].ledger == 51240


async def test_async_watchdog_stats(client):
    stats = await client.watchdog.stats()
    assert stats.healthy == 9
