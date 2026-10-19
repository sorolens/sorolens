# Contract Type Presets

Sorolens ships with built-in presets for common contract types. A preset
pre-configures event filters, function names, and suggested labels so that a
contract can be tracked with one flag instead of manually selecting filters.

## Format

A preset is a JSON object with the following fields:

| Field            | Type     | Description                                                                                                         |
| ---------------- | -------- | ------------------------------------------------------------------------------------------------------------------- |
| `type`           | string   | Machine-readable preset identifier. Used in the CLI `--preset` flag and the API `tracking_preset` field.            |
| `label`          | string   | Human-readable name shown in the dashboard preset selector and CLI help.                                             |
| `description`    | string   | Short paragraph describing what the preset tracks and why a user would pick it.                                     |
| `event_filters`  | string[] | Event-type filters the indexer should prioritise. Used by the dashboard to build filter chips. An empty list means "track all events". |
| `function_names` | string[] | Curated list of function names the preset cares about. Used by the dashboard to surface function-name chips.        |
| `labels`         | string[] | Suggested contract labels the user can apply when tracking. These are suggestions only — the user can type any label. |

### Example

```json
{
  "type": "dex",
  "label": "DEX / AMM",
  "description": "Automated market maker or decentralised exchange contract. Tracks swap, liquidity add/remove, fee, and pool-state events.",
  "event_filters": ["swap", "liquidity_added", "liquidity_removed", "fee_collected", "pool_update"],
  "function_names": ["swap", "add_liquidity", "remove_liquidity", "set_fee", "rebalance"],
  "labels": ["dex", "amm", "liquidity"]
}
```

## Built-in presets

### DEX / AMM (`dex`)

Automated market maker or decentralised exchange contract. Tracks swap,
liquidity add/remove, fee, and pool-state events.

- **Event filters:** `swap`, `liquidity_added`, `liquidity_removed`, `fee_collected`, `pool_update`
- **Function names:** `swap`, `add_liquidity`, `remove_liquidity`, `set_fee`, `rebalance`
- **Suggested labels:** `dex`, `amm`, `liquidity`

### Lending / Borrowing (`lending`)

Lending or borrowing pool contract. Tracks deposit, withdraw, borrow, repay,
liquidation, and interest-accumulation events.

- **Event filters:** `deposit`, `withdraw`, `borrow`, `repay`, `liquidation`, `accrue_interest`, `liquidation_call`
- **Function names:** `deposit`, `withdraw`, `borrow`, `repay`, `liquidate`, `liquidate_account`, `accrue_interest`
- **Suggested labels:** `lending`, `borrowing`, `credit`

### Stablecoin (`stablecoin`)

Stablecoin issuer or peg-maintenance contract. Tracks mint, burn, peg-adjust,
and reserve-report events.

- **Event filters:** `mint`, `burn`, `peg_update`, `reserve_report`, `redeem`
- **Function names:** `mint`, `burn`, `redeem`, `set_peg`, `report_reserves`, `adjust_collateral_ratio`
- **Suggested labels:** `stablecoin`, `peg`, `reserve`

## Using presets

### CLI

```bash
sorolens track CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC \
  --network testnet \
  --preset dex
```

Run `sorolens track --help` to see the available presets and their descriptions.

### API

```bash
curl -X POST http://localhost:8080/api/v1/contracts \
  -H "Content-Type: application/json" \
  -d '{
    "id": "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC",
    "network": "testnet",
    "preset": "dex"
  }'
```

The response includes the `tracking_preset` field:

```json
{
  "id": "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC",
  "network": "testnet",
  "label": "",
  "wasm_hash": "",
  "created_at_ledger": 0,
  "backfill_complete_at": null,
  "status": "pending",
  "added_at": "2026-10-17T12:00:00Z",
  "tags": [],
  "tracking_preset": "dex"
}
```

## Adding new presets

Contributors add new presets by adding an entry to
`cli/internal/presets.BuiltInPresets` and running the existing test suite. The
test `TestBuiltInPresets_Validate` validates every entry, so a malformed preset
fails CI.

For longer-term extensibility, user-defined presets can be stored in a separate
table and merged with the built-in registry at runtime. That work is tracked in
a follow-up issue.

## Validation rules

A preset must satisfy the following:

- `type` must be non-empty.
- `label` must be non-empty.
- `event_filters` entries must be non-empty and unique.
- `function_names` entries must be non-empty and unique.

The built-in presets are validated at test time. User-defined presets will be
validated at registration time when that feature lands.
