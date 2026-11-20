import json
import os

import httpx
import pytest

FIXTURES_DIR = os.path.abspath(
    os.path.join(os.path.dirname(__file__), "../../../tests/sdk-conformance/fixtures")
)

def load_fixtures():
    files = sorted([f for f in os.listdir(FIXTURES_DIR) if f.endswith(".json")])
    fixtures = []
    for f in files:
        with open(os.path.join(FIXTURES_DIR, f)) as fh:
            fixtures.append(json.load(fh))
    return fixtures

FIXTURES = load_fixtures()

def test_fixture_count():
    assert len(FIXTURES) >= 15, f"Expected at least 15 fixtures, got {len(FIXTURES)}"

@pytest.mark.parametrize("fixture", FIXTURES, ids=[f["id"] for f in FIXTURES])
def test_replays_conformance_fixture(fixture):
    status = fixture["response"]["status"]
    headers = fixture["response"]["headers"]
    body = fixture["response"]["body"]
    expected = fixture["expected"]

    # Mock response using httpx Response
    resp = httpx.Response(status_code=status, headers=headers, json=body)
    assert resp.status_code == expected["status"]

    if "retry_after" in expected:
        assert resp.headers.get("Retry-After") == expected["retry_after"]

    data = resp.json()
    if "error_code" in expected:
        assert data.get("error", {}).get("code") == expected["error_code"]

    if "count" in expected:
        has_contracts = "contracts" in data
        count = len(data.get("contracts", [])) if has_contracts else len(data.get("events", []))
        assert count == expected["count"]
