"""Shared pytest fixtures: a recorded-fixture router served through httpx transports."""

from __future__ import annotations

import json
from pathlib import Path

import httpx

import sorolens

FIXTURES = Path(__file__).parent / "fixtures"
BASE_URL = "https://api.test"
API_KEY = "test-key"
CONTRACT_ID = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"


def load(name: str) -> dict:
    """Load a JSON fixture recorded from the Sorolens API."""
    return json.loads((FIXTURES / name).read_text())


def api_handler(request: httpx.Request) -> httpx.Response:
    """Serve recorded fixtures based on the request method and path."""
    method, path = request.method, request.url.path
    routes: dict[tuple[str, str], tuple[int, str]] = {
        ("GET", "/api/v1/stats/global"): (200, "global_stats.json"),
        ("GET", "/api/v1/contracts"): (200, "contracts_page.json"),
        ("GET", f"/api/v1/contracts/{CONTRACT_ID}"): (200, "contract.json"),
        ("GET", "/api/v1/contracts/missing"): (404, "error_not_found.json"),
        ("GET", f"/api/v1/contracts/{CONTRACT_ID}/events"): (200, "events_page.json"),
        ("GET", "/api/v1/watchdog/stats"): (200, "watchdog_stats.json"),
        ("POST", "/api/v1/api-keys"): (201, "create_api_key.json"),
    }
    if (method, path) in routes:
        status, fixture = routes[(method, path)]
        return httpx.Response(status, json=load(fixture))
    return httpx.Response(
        404,
        json={"error": {"code": "NOT_FOUND", "message": f"no fixture for {method} {path}"}},
    )


def make_sync_client(transport: httpx.BaseTransport | None = None) -> sorolens.Client:
    """Build a sync client whose requests are served by the fixture router."""
    return sorolens.Client(
        api_key=API_KEY,
        base_url=BASE_URL,
        httpx_args={"transport": transport or httpx.MockTransport(api_handler)},
    )


def make_async_client(transport: httpx.AsyncBaseTransport | None = None) -> sorolens.AsyncClient:
    """Build an async client whose requests are served by the fixture router."""
    return sorolens.AsyncClient(
        api_key=API_KEY,
        base_url=BASE_URL,
        httpx_args={"transport": transport or httpx.MockTransport(api_handler)},
    )
