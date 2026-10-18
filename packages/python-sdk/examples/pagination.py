"""Cursor pagination against ``GET /api/v1/contracts``.

Run against a live server::

    SOROLENS_API_URL=https://api.sorolens.xyz python examples/pagination.py

Without ``SOROLENS_API_URL`` the example serves canned pages through an
``httpx.MockTransport`` and asserts the full walk, so it is runnable offline
(that is what CI does).
"""

from __future__ import annotations

import os

import httpx

from sorolens import Client

PAGE_SIZE = 2

PAGE_1 = {
    "contracts": [
        {
            "id": "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC",
            "network": "testnet",
            "label": "counter",
            "wasm_hash": "9d3f2a1b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f7",
            "created_at_ledger": 51234,
            "backfill_complete_at": "2026-01-15T12:00:00Z",
            "status": "active",
            "added_at": "2026-01-15T11:30:00Z",
            "tags": ["prod"],
        },
        {
            "id": "CBECJ5JYFLYQZQWZMFIIWOJHTBLXEWQOGXFVN5BPOVQTQFSXJPWBBV6",
            "network": "mainnet",
            "label": "watchdog",
            "wasm_hash": "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff",
            "created_at_ledger": 90001,
            "backfill_complete_at": None,
            "status": "backfilling",
            "added_at": "2026-02-01T08:15:00Z",
            "tags": [],
        },
    ],
    "next_cursor": "page-2",
}

PAGE_2 = {
    "contracts": [
        {
            "id": "CAQYAADZS2Y4XG4FWKF6MUNQCVKJXHVQEBDGV2Y4XG4FWKF6MUNQCWEB",
            "network": "futurenet",
            "label": "vault",
            "wasm_hash": None,
            "created_at_ledger": 120001,
            "backfill_complete_at": None,
            "status": "active",
            "added_at": "2026-03-04T09:00:00Z",
            "tags": [],
        },
    ],
    "next_cursor": "",
}


def offline_handler(request: httpx.Request) -> httpx.Response:
    cursor = request.url.params.get("cursor")
    page = PAGE_2 if cursor == "page-2" else PAGE_1
    return httpx.Response(200, json=page)


def build_client() -> Client:
    base_url = os.environ.get("SOROLENS_API_URL")
    if base_url:
        return Client(base_url=base_url, api_key=os.environ.get("SOROLENS_API_KEY"))
    return Client(
        base_url="https://offline.invalid",
        httpx_args={"transport": httpx.MockTransport(offline_handler)},
    )


def walk_contracts(client: Client) -> list[str]:
    """Follow ``next_cursor`` until the API reports the last page."""
    contract_ids: list[str] = []
    cursor: str | None = None
    while True:
        page = client.contracts.list(limit=PAGE_SIZE, cursor=cursor)
        contract_ids.extend(contract.id for contract in page.contracts)
        if not page.next_cursor:
            return contract_ids
        cursor = page.next_cursor


def main() -> int:
    offline = not os.environ.get("SOROLENS_API_URL")
    with build_client() as client:
        contract_ids = walk_contracts(client)

    print(f"paginated through {len(contract_ids)} contracts")
    for contract_id in contract_ids:
        print(f"  {contract_id}")

    if offline:
        assert len(contract_ids) == 3, contract_ids
        assert len(set(contract_ids)) == 3, "cursor pages must not repeat items"
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
