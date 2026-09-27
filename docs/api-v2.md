# API v2

Issue #144 introduces `/api/v2/*` alongside the existing `/api/v1/*`. Nothing in
v1 changes: v2 exists so that the inconsistencies v1 accumulated can be fixed
without breaking existing clients.

- [Why v2](#why-v2)
- [v2 conventions](#v2-conventions)
- [Route map](#route-map)
- [Field-by-field mapping](#field-by-field-mapping)
- [Query parameter changes](#query-parameter-changes)
- [Migrating a client](#migrating-a-client)
- [What is not yet v2-native](#what-is-not-yet-v2-native)

---

## Why v2

The v1 surface grew endpoint by endpoint and the seams show:

| Inconsistency | v1 behavior |
|---|---|
| List envelope | `{"contracts": [...]}`, `{"events": [...]}`, `{"storage": [...]}`, `{"health_checks": [...]}`, `{"alerts": [...]}` — a different item key per resource |
| Pagination | Some list endpoints take a `cursor`, some take none; the cursor is returned sometimes as `cursor`, sometimes as `next_cursor`, and sometimes not at all |
| Timestamps | Mostly RFC 3339, but the contract is undocumented and numeric/epoch values have leaked into newer fields |
| Number naming | `cpu_insn`, `mem_byte`, `ledger_read_byte`, `ledger_write_byte` — abbreviations with inconsistent units |
| Absent fields | Optional values are either omitted or returned as `""`, so clients cannot tell "absent" from "empty string" |

v2 fixes all five at once.

---

## v2 conventions

1. **One list envelope.** Every list endpoint returns:

   ```json
   {
     "data": [ ... ],
     "pagination": { "next_cursor": "MDAwMDg1...", "has_more": true }
   }
   ```

   The item key is always `data`. On the last page `has_more` is `false` and
   `next_cursor` is `null`.

2. **Every list endpoint paginates.** `cursor` and `limit` are accepted
   everywhere, and `pagination` is always present — including on an empty
   result.

3. **Every timestamp is RFC 3339 UTC.** v2 never returns a unix epoch or a
   non-UTC offset. Nullable timestamps stay present as an explicit `null`.

4. **Optional fields are always present.** A missing value is `null`, never an
   omitted key and never `""`.

5. **Nested arrays keep their semantic name.** `data` names *the page*. Arrays
   inside a single-resource document keep the name that describes them
   (`storage`, `series`, `events`) because they are not the page.

6. **Errors are unchanged.** Both namespaces return
   `{"error": {"code", "message", "request_id"}}` with the same status codes,
   so error handling does not need to branch on the version.

---

## Route map

Every v1 route has a v2 counterpart at the same path with `/api/v1` replaced by
`/api/v2`. This is enforced by `TestV2CoversEveryV1Route`, which walks the live
chi route table and fails if a v1 route has no v2 equivalent.

| Method | v1 | v2 |
|---|---|---|
| GET | `/api/v1/stats/global` | `/api/v2/stats/global` |
| GET | `/api/v1/stats/activity` | `/api/v2/stats/activity` |
| GET | `/api/v1/events/recent` | `/api/v2/events/recent` |
| POST | `/api/v1/contracts` | `/api/v2/contracts` |
| GET | `/api/v1/contracts` | `/api/v2/contracts` |
| GET | `/api/v1/contracts/{id}` | `/api/v2/contracts/{id}` |
| GET | `/api/v1/contracts/{id}/events` | `/api/v2/contracts/{id}/events` |
| GET | `/api/v1/contracts/{id}/invocations` | `/api/v2/contracts/{id}/invocations` |
| GET | `/api/v1/contracts/{id}/storage` | `/api/v2/contracts/{id}/storage` |
| GET | `/api/v1/contracts/{id}/stats` | `/api/v2/contracts/{id}/stats` |
| GET | `/api/v1/contracts/{id}/forecast` | `/api/v2/contracts/{id}/forecast` |
| GET | `/api/v1/contracts/{id}/snapshot` | `/api/v2/contracts/{id}/snapshot` |
| GET | `/api/v1/contracts/{id}/upgrades` | `/api/v2/contracts/{id}/upgrades` |
| GET | `/api/v1/contracts/{id}/health-score` | `/api/v2/contracts/{id}/health-score` |
| GET | `/api/v1/contracts/{id}/stream` | `/api/v2/contracts/{id}/stream` |
| GET | `/api/v1/contracts/{id}/graph` | `/api/v2/contracts/{id}/graph` |
| GET/POST/DELETE | `/api/v1/api-keys`, `/api/v1/api-keys/{id}` | `/api/v2/api-keys`, `/api/v2/api-keys/{id}` |
| GET/POST/DELETE | `/api/v1/watchlist`, `/api/v1/watchlist/{contractId}`, `/api/v1/watchlist/{contractId}/status` | `/api/v2/...` |
| GET | `/api/v1/watchdog/stats` | `/api/v2/watchdog/stats` |
| GET | `/api/v1/watchdog/alerts` | `/api/v2/watchdog/alerts` |
| GET | `/api/v1/watchdog/contracts` | `/api/v2/watchdog/contracts` |
| GET | `/api/v1/watchdog/contracts/{id}` | `/api/v2/watchdog/contracts/{id}` |
| GET | `/api/v1/watchdog/contracts/{id}/health` | `/api/v2/watchdog/contracts/{id}/health` |
| GET | `/api/v1/watchdog/contracts/{id}/alerts` | `/api/v2/watchdog/contracts/{id}/alerts` |
| GET/POST/DELETE | `/api/v1/admin/keys` | `/api/v2/admin/keys` |

Scopes and roles are identical across both namespaces: a key that can read
`/api/v1/contracts` can read `/api/v2/contracts`, and the same `admin` role
gates the admin surface in both.

---

## Field-by-field mapping

### List envelope

| Concept | v1 | v2 |
|---|---|---|
| Items | `contracts` / `events` / `invocations` / `storage` / `health_checks` / `alerts` | `data` |
| Next page token | `cursor` on some endpoints, `next_cursor` on others, absent on the rest | `pagination.next_cursor` (always present; `null` on the last page) |
| More pages? | not returned | `pagination.has_more` |

### Contract

| v1 | v2 | Note |
|---|---|---|
| `id` | `id` | |
| `network` | `network` | |
| `label` | `label` | `""` in v1 becomes `null` in v2 |
| `wasm_hash` | `wasm_hash` | `""` in v1 becomes `null` in v2 |
| `created_at_ledger` | `created_at_ledger` | |
| `backfill_complete_at` | `backfill_complete_at` | RFC 3339 or `null` |
| `status` | `status` | |
| `added_at` | `added_at` | RFC 3339 |

### Event

| v1 | v2 |
|---|---|
| `id`, `contract_id`, `network`, `ledger`, `tx_hash`, `type` | unchanged |
| `ledger_closed_at` | `ledger_closed_at` — guaranteed RFC 3339 UTC |
| `topic_xdr`, `value_xdr` | unchanged |
| `topic_decoded`, `value_decoded` | `[]` / `null` rather than omitted |
| `in_successful_call` | unchanged |

### Invocation — the renames

| v1 | v2 | Why |
|---|---|---|
| `resource_fee_charged` | `resource_fee_charged_stroops` | v1 never stated the unit; v2 names it |
| `cpu_insn` | `cpu_instructions` | spelled out |
| `mem_byte` | `memory_bytes` | unit corrected and pluralized |
| `ledger_read_byte` | `ledger_read_bytes` | pluralized |
| `ledger_write_byte` | `ledger_write_bytes` | pluralized |
| `function_name` | `function_name` | `""` in v1 becomes `null` in v2 |

All other invocation fields are unchanged.

### Storage entry

| v1 | v2 |
|---|---|
| `key_xdr`, `key_decoded`, `value_xdr`, `value_decoded`, `durability`, `status` | unchanged |
| `live_until_ledger`, `last_modified_ledger` | unchanged |
| `last_seen_at` | `last_seen_at` — guaranteed RFC 3339 UTC |

### Watchdog

| v1 | v2 |
|---|---|
| `contracts` (list) | `data` |
| `health_checks` (list) | `data` |
| `alerts` (list) | `data` |
| `last_check` | `last_check` — RFC 3339 or `null` |
| `registered_at`, `updated_at`, `timestamp` | RFC 3339 UTC |

`watchdog/stats` is an object, not a list, so its fields are unchanged.

### Upgrades

| v1 | v2 |
|---|---|
| `upgrades` (list) | `data` |
| `tx_hash` | `tx_hash` — `omitempty` in v1, explicit `null` in v2 |
| `at` | `at` — guaranteed RFC 3339 UTC |

### Live feeds

| v1 | v2 |
|---|---|
| `/stats/activity` returns `contracts` | returns `data` |
| `window_start` | `window_start` — RFC 3339 UTC |
| `per_minute` | unchanged (contiguous buckets, oldest first) |

---

## Query parameter changes

| Endpoint | v1 | v2 |
|---|---|---|
| `contracts/{id}/invocations` | `fn` | `function_name` (v2 also accepts `fn` for a migration window) |
| all list endpoints | `cursor`, `limit` on some | `cursor`, `limit` on all |

All other query parameters (`network`, `status`, `type`, `from`, `to`,
`durability`, `severity`, `window`, `horizon`, `ledger`) are unchanged.

---

## Migrating a client

1. Replace `/api/v1` with `/api/v2` in the base URL.
2. Replace the per-resource item key with `data`:

   ```diff
   - for (const e of body.events) { ... }
   + for (const e of body.data) { ... }
   ```

3. Replace cursor handling:

   ```diff
   - const next = body.next_cursor;   // may be undefined
   + const next = body.pagination.next_cursor; // always present
   - while (next) { ... }             // no has_more in v1
   + while (body.pagination.has_more) { ... }
   ```

4. Rename the four invocation resource fields (`cpu_insn` →
   `cpu_instructions`, `mem_byte` → `memory_bytes`, `ledger_read_byte` →
   `ledger_read_bytes`, `ledger_write_byte` → `ledger_write_bytes`) and
   `resource_fee_charged` → `resource_fee_charged_stroops`.

5. Treat optional fields as `null` rather than absent. `label`,
   `wasm_hash`, `function_name`, `tx_hash`, `backfill_complete_at`, and
   `last_check` are always present in v2.

---

## What is not yet v2-native

These routes exist under `/api/v2` and are fully supported, but they delegate
to the v1 handler rather than to a v2-specific DTO. Their responses already
satisfy the v2 conventions (RFC 3339 timestamps, explicit nulls, stable field
names, and, for `stream`, a bounded non-paginated list), so there is nothing
left to change; the delegation is declared here rather than left implicit.

| Route | Reason |
|---|---|
| `/api/v2/contracts/{id}/forecast` | Nested `series[].points[]` document, already RFC 3339 (`date`) and explicitly typed |
| `/api/v2/contracts/{id}/snapshot` | Single-resource document; nested `storage` array keeps its semantic name per convention 5 |
| `/api/v2/contracts/{id}/stream` | Bounded list of the 20 newest events, not a page, so `pagination` does not apply |
| `/api/v2/contracts/{id}/graph` | Node/edge document with stable `id`/`from`/`to` fields |
| `/api/v2/api-keys`, `/api/v2/admin/keys` | Admin surface; the v1 document is already minimal (`id`, `name`, `key_prefix`, `scopes`, timestamps) |

Everything else uses a native v2 DTO.
