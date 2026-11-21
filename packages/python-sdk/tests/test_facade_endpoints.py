"""Tests for the facade resources added for the remaining OpenAPI operations.

Covers alerts, search, labels, comparisons, per-contract insights (upgrades,
call graph, health score, summary), the DLQ, reports and notification
subscriptions. Every request is served by :class:`httpx.MockTransport`, so the
tests exercise the real parsing and error-mapping paths without a network.
"""

from __future__ import annotations

import json

import httpx
import pytest

import sorolens
from conftest import CONTRACT_ID, make_async_client, make_sync_client
from sorolens import exceptions


def contract(**overrides: object) -> dict:
    payload: dict[str, object] = {
        "id": CONTRACT_ID,
        "network": "testnet",
        "label": "counter",
        "wasm_hash": "3c1b2d9f",
        "created_at_ledger": 119_000,
        "backfill_complete_at": None,
        "status": "active",
        "added_at": "2026-11-01T00:00:00Z",
        "tags": ["prod"],
    }
    payload.update(overrides)
    return payload


def compare_entry(contract_id: str, **overrides: object) -> dict:
    payload: dict[str, object] = {
        "id": contract_id,
        "network": "testnet",
        "label": "counter",
        "status": "active",
        "tracked": True,
        "has_data": True,
        "event_count": 10,
        "invocation_count": 4,
        "avg_cpu": 1200.5,
        "avg_fee": 30.0,
        "last_synced_ledger": 120_400,
        "event_volume": [{"timestamp": "2026-11-01T00:00:00Z", "count": 2}],
        "health_score": 92,
        "health_components": {
            "uptime": 95,
            "error_rate": 90,
            "performance": 88,
            "storage_ttl": 100,
        },
    }
    payload.update(overrides)
    return payload


def monthly_sla(**overrides: object) -> dict:
    payload: dict[str, object] = {
        "contract_id": CONTRACT_ID,
        "month": "2026-11",
        "uptime_pct": 99.5,
        "total_checks": 200,
        "healthy_checks": 199,
        "incidents": 1,
        "mttr_seconds": 120.0,
        "total_downtime_seconds": 120.0,
        "longest_outage_seconds": 120.0,
        "ongoing_outage": False,
        "critical_alerts": 0,
        "warning_alerts": 1,
        "info_alerts": 2,
        "total_alerts": 3,
        "first_check": "2026-11-01T00:00:00Z",
        "last_check": "2026-11-02T00:00:00Z",
    }
    payload.update(overrides)
    return payload


def alert_subscription(**overrides: object) -> dict:
    payload: dict[str, object] = {
        "id": "sub_1",
        "contract_id": CONTRACT_ID,
        "channel_type": "slack",
        "webhook_url": "https://hooks.slack.com/services/***",
        "has_routing_key": False,
        "severity_filter": "Critical",
        "created_at": "2026-11-01T00:00:00Z",
        "updated_at": "2026-11-01T00:00:00Z",
    }
    payload.update(overrides)
    return payload


def handler(request: httpx.Request) -> httpx.Response:
    """Serve a response for every operation this module exercises."""
    method, path = request.method, request.url.path

    if method == "GET" and path == "/api/v1/alerts":
        return httpx.Response(
            200,
            json={
                "next_cursor": "",
                "groups": [
                    {
                        "id": 1,
                        "group_key": f"{CONTRACT_ID}|Critical|",
                        "contract_id": CONTRACT_ID,
                        "severity": "Critical",
                        "rule": "",
                        "count": 3,
                        "dedupe_window_secs": 300,
                        "first_seen": "2026-11-01T00:00:00Z",
                        "last_seen": "2026-11-01T00:10:00Z",
                        "last_message": "storage entry expiring",
                        "backfill_eligible": False,
                    }
                ],
            },
        )
    if method == "GET" and path == "/api/v1/search":
        return httpx.Response(200, json={"items": [contract()]})
    if method == "GET" and path == "/api/v1/labels":
        return httpx.Response(200, json={"labels": [{"label": "counter", "value": CONTRACT_ID}]})
    if method == "POST" and path == "/api/v1/labels":
        return httpx.Response(201, json={"label": "counter", "value": CONTRACT_ID})
    if method == "GET" and path == "/api/v1/resolve":
        if request.url.params.get("query") == "missing":
            return httpx.Response(
                404,
                json={"error": {"code": "NOT_FOUND", "message": "label not found"}},
            )
        return httpx.Response(200, json={"label": "counter", "value": CONTRACT_ID})
    if method == "GET" and path == "/api/v1/compare":
        ids = request.url.params.get("ids", "")
        return httpx.Response(
            200,
            json={
                "window": request.url.params.get("window", "24h"),
                "contracts": [compare_entry(contract_id) for contract_id in ids.split(",")],
            },
        )
    if method == "GET" and path == f"/api/v1/contracts/{CONTRACT_ID}/upgrades":
        return httpx.Response(
            200,
            json={
                "upgrades": [
                    {
                        "contract_id": CONTRACT_ID,
                        "from_hash": "aa",
                        "to_hash": "bb",
                        "ledger": 120_400,
                        "at": "2026-11-01T00:00:00Z",
                        "tx_hash": "a1b2c3",
                    }
                ]
            },
        )
    if method == "GET" and path == f"/api/v1/contracts/{CONTRACT_ID}/graph":
        return httpx.Response(
            200,
            json={
                "nodes": [{"id": CONTRACT_ID, "label": "counter"}],
                "edges": [{"source": CONTRACT_ID, "target": CONTRACT_ID, "count": 2}],
            },
        )
    if method == "GET" and path == f"/api/v1/contracts/{CONTRACT_ID}/health-score":
        return httpx.Response(
            200,
            json={
                "contract_id": CONTRACT_ID,
                "score": 92,
                "components": {
                    "uptime": 95,
                    "error_rate": 90,
                    "performance": 88,
                    "storage_ttl": 100,
                },
                "computed_at": "2026-11-01T00:00:00Z",
            },
        )
    if method == "GET" and path == f"/api/v1/contracts/{CONTRACT_ID}/summary":
        return httpx.Response(
            200,
            json={
                "contract_id": CONTRACT_ID,
                "network": "testnet",
                "label": "counter",
                "status": "active",
                "generated_at": "2026-11-01T00:00:00Z",
                "stats": {
                    "event_count": 10,
                    "invocation_count": 4,
                    "storage_count": 2,
                    "last_synced_ledger": 120_400,
                    "window_event_count": 10,
                    "window_invocation_count": 4,
                    "window_duration": "7d",
                },
            },
        )
    if method == "GET" and path == "/api/v1/contracts/missing/summary":
        return httpx.Response(
            404, json={"error": {"code": "NOT_FOUND", "message": "contract not found"}}
        )
    if method == "GET" and path == "/api/v1/dlq":
        return httpx.Response(
            200,
            json={
                "items": [
                    {
                        "id": 7,
                        "event_id": "evt_7",
                        "contract_id": CONTRACT_ID,
                        "network": "testnet",
                        "error_message": "decode failed",
                        "attempts": 5,
                        "created_at": "2026-11-01T00:00:00Z",
                    }
                ],
                "next_cursor": "",
            },
        )
    if method == "POST" and path == "/api/v1/dlq/7/requeue":
        return httpx.Response(
            200, json={"id": 7, "event_id": "evt_7", "status": "requeued"}
        )
    if method == "POST" and path == "/api/v1/dlq/404/requeue":
        return httpx.Response(
            404, json={"error": {"code": "NOT_FOUND", "message": "dlq row not found"}}
        )
    if method == "GET" and path == "/api/v1/reports/subscriptions":
        return httpx.Response(
            200,
            json={
                "subscriptions": [
                    {
                        "id": "rsub_1",
                        "email": "ops@example.com",
                        "frequency": "weekly",
                        "day_of_week": 1,
                        "created_at": "2026-11-01T00:00:00Z",
                    }
                ]
            },
        )
    if method == "POST" and path == "/api/v1/reports/subscriptions":
        return httpx.Response(
            201,
            json={
                "id": "rsub_2",
                "email": "ops@example.com",
                "frequency": "daily",
                "day_of_week": 0,
                "created_at": "2026-11-01T00:00:00Z",
            },
        )
    if method == "DELETE" and path == "/api/v1/reports/subscriptions/rsub_1":
        return httpx.Response(204)
    if method == "GET" and path == "/api/v1/reports/unsubscribe":
        return httpx.Response(200, text="unsubscribed")
    if method == "GET" and path == f"/api/v1/reports/{CONTRACT_ID}/badge.svg":
        return httpx.Response(200, text="<svg>badge</svg>")
    if method == "GET" and path == f"/api/v1/reports/{CONTRACT_ID}/history":
        return httpx.Response(
            200,
            json={"contract_id": CONTRACT_ID, "months": [monthly_sla()]},
        )
    if method == "GET" and path == f"/api/v1/reports/{CONTRACT_ID}":
        return httpx.Response(200, json=monthly_sla(signed=False))
    if method == "GET" and path == "/api/v1/reports/bad-contract":
        return httpx.Response(
            422, json={"error": {"code": "INVALID_INPUT", "message": "invalid month"}}
        )
    if method == "GET" and path == "/api/v1/watchdog/subscriptions":
        return httpx.Response(
            200, json={"subscriptions": [alert_subscription()]}
        )
    if method == "POST" and path == "/api/v1/watchdog/subscriptions":
        body = json.loads(request.content)
        return httpx.Response(
            201, json=alert_subscription(channel_type=body["channel_type"])
        )
    if method == "DELETE" and path == "/api/v1/watchdog/subscriptions/sub_1":
        return httpx.Response(204)
    if method == "DELETE" and path == "/api/v1/watchdog/subscriptions/forbidden":
        return httpx.Response(
            403,
            json={"error": "Forbidden", "required": "admin", "role": "contributor"},
        )
    if method == "GET" and path == "/api/v1/alerts-broken":
        return httpx.Response(500, json={"error": "boom"})
    return httpx.Response(
        404, json={"error": {"code": "NOT_FOUND", "message": f"no route for {method} {path}"}}
    )


def sync_client() -> sorolens.Client:
    return make_sync_client(httpx.MockTransport(handler))


def test_alerts_list_groups():
    with sync_client() as client:
        page = client.alerts.list(severity="Critical", network="testnet", limit=10)
    assert page.next_cursor == ""
    assert page.groups[0].severity.value == "Critical"
    assert page.groups[0].count == 3


def test_alerts_error_maps_to_api_error():
    def failing(request: httpx.Request) -> httpx.Response:
        return httpx.Response(500, json={"error": {"message": "boom"}})

    with make_sync_client(httpx.MockTransport(failing)) as client:
        with pytest.raises(exceptions.APIError) as exc_info:
            client.alerts.list()
    assert exc_info.value.status_code == 500


def test_search_contracts():
    with sync_client() as client:
        page = client.search.contracts("counter", limit=5)
    assert [item.id for item in page.items] == [CONTRACT_ID]


def test_labels_round_trip():
    with sync_client() as client:
        listed = client.labels.list(query="coun")
        created = client.labels.create("counter", CONTRACT_ID, scope="workspace")
        resolved = client.labels.resolve("counter")
    assert listed["labels"][0]["label"] == "counter"
    assert created["value"] == CONTRACT_ID
    assert resolved["value"] == CONTRACT_ID


def test_resolve_missing_label_raises_not_found():
    with sync_client() as client:
        with pytest.raises(exceptions.NotFoundError):
            client.labels.resolve("missing")


def test_compare_contracts():
    with sync_client() as client:
        comparison = client.stats.compare([CONTRACT_ID, "C" + "A" * 55], window="7d")
    assert comparison.window.value == "7d"
    assert [entry.id for entry in comparison.contracts] == [CONTRACT_ID, "C" + "A" * 55]
    assert comparison.contracts[0].health_score == 92


def test_contract_insights():
    with sync_client() as client:
        upgrades = client.contracts.upgrades(CONTRACT_ID, limit=10)
        graph = client.contracts.graph(CONTRACT_ID)
        score = client.contracts.health_score(CONTRACT_ID, history_limit=24)
        summary = client.contracts.summary(CONTRACT_ID)
    assert upgrades.upgrades[0].to_hash == "bb"
    assert graph.edges[0].count == 2
    assert score.score == 92
    assert summary.stats.invocation_count == 4


def test_missing_summary_raises_not_found():
    with sync_client() as client:
        with pytest.raises(exceptions.NotFoundError) as exc_info:
            client.contracts.summary("missing")
    assert "contract not found" in str(exc_info.value)


def test_dlq_list_and_requeue():
    with sync_client() as client:
        page = client.dlq.list(limit=10)
        requeued = client.dlq.requeue(7)
    assert page.items[0].attempts == 5
    assert requeued.status == "requeued"
    with sync_client() as client:
        with pytest.raises(exceptions.NotFoundError):
            client.dlq.requeue(404)


def test_reports():
    with sync_client() as client:
        report = client.reports.contract(CONTRACT_ID, month="2026-11")
        history = client.reports.history(CONTRACT_ID, months=3)
        badge = client.reports.badge(CONTRACT_ID, month="2026-11")
        created = client.reports.create_subscription(
            "ops@example.com", "daily"
        )
        subscriptions = client.reports.subscriptions("ops@example.com")
        client.reports.delete_subscription("rsub_1")
        text = client.reports.unsubscribe("token")
    assert report.uptime_pct == 99.5
    assert history.months[0].incidents == 1
    assert badge == "<svg>badge</svg>"
    assert created.frequency.value == "daily"
    assert subscriptions.subscriptions[0].id == "rsub_1"
    assert text == "unsubscribed"


def test_report_error_maps_to_validation_error():
    with sync_client() as client:
        with pytest.raises(exceptions.ValidationError):
            client.reports.contract("bad-contract")


def test_notifications():
    with sync_client() as client:
        subscriptions = client.notifications.list()
        created = client.notifications.create(
            CONTRACT_ID,
            channel_type="discord",
            webhook_url="https://discord.com/api/webhooks/1",
        )
        client.notifications.delete("sub_1")
    assert subscriptions.subscriptions[0].severity_filter.value == "Critical"
    assert created.channel_type.value == "discord"


def test_notification_forbidden_maps_to_forbidden_error():
    with sync_client() as client:
        with pytest.raises(exceptions.ForbiddenError):
            client.notifications.delete("forbidden")


async def test_async_facade_endpoints():
    client = make_async_client(httpx.MockTransport(handler))
    try:
        alerts = await client.alerts.list(flat=True)
        comparison = await client.stats.compare(CONTRACT_ID)
        subscriptions = await client.notifications.list()
    finally:
        await client.close()
    assert alerts.groups[0].last_message == "storage entry expiring"
    assert comparison.window.value == "24h"
    assert subscriptions.subscriptions[0].id == "sub_1"


async def test_async_contract_insights_and_labels():
    client = make_async_client(httpx.MockTransport(handler))
    try:
        score = await client.contracts.health_score(CONTRACT_ID)
        labels = await client.labels.list()
        with pytest.raises(exceptions.NotFoundError):
            await client.contracts.summary("missing")
    finally:
        await client.close()
    assert score.score == 92
    assert labels["labels"][0]["value"] == CONTRACT_ID
