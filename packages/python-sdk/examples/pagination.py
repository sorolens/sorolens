"""Walk every page of ``GET /api/v1/contracts`` with the opaque ``next_cursor``.

This is the pattern to follow for any list endpoint: pass the cursor from the
previous page until the API returns an empty one.

Against a deployment::

    SOROLENS_BASE_URL=http://localhost:8080 SOROLENS_API_KEY=sl_dev_... \\
        python examples/pagination.py

Offline self-check (what CI runs)::

    python examples/pagination.py --self-test
"""

from __future__ import annotations

import argparse
import os

import httpx

import sorolens

PAGE_SIZE = 2


def walk(client: sorolens.Client) -> list[str]:
    """Return every contract id, following ``next_cursor`` until it is empty."""
    ids: list[str] = []
    cursor: str | None = None
    while True:
        page = client.contracts.list(limit=PAGE_SIZE, cursor=cursor)
        for contract in page.contracts:
            print(contract.id, contract.network, contract.status.value)
            ids.append(contract.id)
        if not page.next_cursor:
            return ids
        cursor = page.next_cursor


def _contract(suffix: str, network: str, status: str) -> dict:
    return {
        "id": "C" + "A" * 53 + suffix,
        "network": network,
        "label": f"contract-{suffix.lower()}",
        "wasm_hash": "00" * 32,
        "created_at_ledger": 100,
        "backfill_complete_at": None,
        "status": status,
        "added_at": "2026-08-01T00:00:00Z",
        "tags": [],
    }


def _mock_api(request: httpx.Request) -> httpx.Response:
    """Serve two pages: the second is only reachable through the first page's cursor."""
    if request.url.params.get("cursor") == "cursor-page-2":
        contracts = [_contract("D4", "testnet", "active"), _contract("E5", "mainnet", "paused")]
        return httpx.Response(200, json={"contracts": contracts, "next_cursor": ""})
    contracts = [_contract("H2", "testnet", "active"), _contract("C3", "mainnet", "backfilling")]
    return httpx.Response(200, json={"contracts": contracts, "next_cursor": "cursor-page-2"})


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--self-test", action="store_true", help="run against a built-in mock")
    args = parser.parse_args(argv)

    if args.self_test:
        client = sorolens.Client(
            base_url="https://api.test", httpx_args={"transport": httpx.MockTransport(_mock_api)}
        )
        with client:
            ids = walk(client)
        if len(ids) != 4:
            print(f"self-test expected 4 contracts, got {len(ids)}")
            return 1
        print(f"self-test: paged through {len(ids)} contracts")
        return 0

    base_url = os.environ.get("SOROLENS_BASE_URL", sorolens.DEFAULT_BASE_URL)
    with sorolens.Client(base_url=base_url, api_key=os.environ.get("SOROLENS_API_KEY")) as client:
        ids = walk(client)
    print(f"fetched {len(ids)} contract(s)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
