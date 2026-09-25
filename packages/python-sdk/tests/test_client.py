"""Synchronous client tests against recorded API fixtures."""

from __future__ import annotations

import httpx
import pytest

from conftest import API_KEY, BASE_URL, CONTRACT_ID, make_sync_client
from sorolens import exceptions


@pytest.fixture
def client():
    http_client = make_sync_client()
    yield http_client
    http_client.close()


def test_global_stats(client):
    stats = client.stats.global_stats()
    assert stats.tracked_contracts == 42
    assert stats.total_events == 1048576


def test_list_contracts(client):
    page = client.contracts.list(limit=2)
    assert page.next_cursor == "eyJvZmZzZXQiOjJ9"
    assert [contract.label for contract in page.contracts] == ["counter", "watchdog"]
    assert page.contracts[1].backfill_complete_at is None


def test_get_contract(client):
    contract = client.contracts.get(CONTRACT_ID)
    assert contract.id == CONTRACT_ID
    assert contract.status.value == "active"


def test_get_missing_contract_raises_not_found(client):
    with pytest.raises(exceptions.NotFoundError) as exc_info:
        client.contracts.get("missing")
    assert "contract not found" in str(exc_info.value)
    assert exc_info.value.status_code == 404


def test_list_events(client):
    page = client.events.list(CONTRACT_ID, limit=1)
    assert page.events[0].tx_hash.startswith("a1b2c3")


def test_watchdog_stats_filters(client):
    stats = client.watchdog.stats(network="mainnet")
    assert stats.total_monitored == 12
    assert stats.critical_alerts == 3


def test_create_api_key(client):
    created = client.api_keys.create("ci-reader", ["read:contracts"])
    assert created.key == "sk_read_0123456789abcdef0123456789abcdef"
    assert created.name == "ci-reader"


def test_api_key_sent_as_bearer_token():
    seen: list[httpx.Request] = []

    def transport(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"status": "ok"})

    with make_sync_client(httpx.MockTransport(transport)) as client:
        client.health.health()

    assert seen[0].headers["Authorization"] == f"Bearer {API_KEY}"
    assert str(seen[0].url) == f"{BASE_URL}/health"
