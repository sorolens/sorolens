# RESEARCH.md
## Sorolens Pre-Build Research

All URLs accessed on 2026-07-26.

---

## 1. Soroban RPC JSON-RPC Methods

> Note: "Soroban-RPC" was renamed to "Stellar RPC" in November 2024. The rename is branding only; the JSON-RPC wire protocol and endpoint URLs did not change. Both names appear in docs and third-party resources.

### 1.1 General Request Envelope

All methods share the JSON-RPC 2.0 envelope. Parameters must be passed as a named object, not a positional array.

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "<methodName>",
  "params": { ... }
}
```

---

### 1.2 `getEvents`

**Source:** https://developers.stellar.org/docs/data/apis/rpc/api-reference/methods/getEvents  
**Accessed:** 2026-07-26

Fetches a filtered list of events emitted within a ledger range. RPC retains events for approximately 24 hours by default, with a maximum query window of 7 days.

**Request params:**

| Field | Type | Notes |
|---|---|---|
| `startLedger` | number | Inclusive start ledger. Omit when using cursor. |
| `endLedger` | number | Exclusive end ledger. Omit when using cursor. |
| `filters` | array (max 5) | Each filter has `type`, `contractIds` (max 5), and `topics` (max 5 per filter). |
| `pagination.cursor` | string | Opaque paging token. When present, `startLedger`/`endLedger` must be omitted. |
| `pagination.limit` | number | 1-10000, default 100. |
| `xdrFormat` | string | `"base64"` (default) or `"json"`. |

Filter `type` values: `"contract"` or `"system"`.  
Topic segments are base64-encoded XDR `ScVal` strings. `"*"` matches any single segment; `"**"` matches any remaining segments.

**Response:**

```json
{
  "jsonrpc": "2.0",
  "id": 8675309,
  "result": {
    "events": [
      {
        "type": "contract",
        "ledger": 200010,
        "ledgerClosedAt": "2025-06-30T07:27:13Z",
        "contractId": "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC",
        "id": "0000859036408881152-0000000003",
        "pagingToken": "0000859036408881152-0000000003",
        "inSuccessfulContractCall": true,
        "txHash": "d9e771ac73ec80503c7594f540d10ec068fb80981d11acea41aa193b7543c5ce",
        "topic": ["AAAADwAAAAh0cmFuc2Zlcg==", "AAAA..."],
        "value": "AAAACgAAAA...",
        "transactionIndex": 3,
        "operationIndex": 0
      }
    ],
    "latestLedger": 320543,
    "cursor": "0000863490289963008-0000000010"
  }
}
```

Key notes:
- `id` is a TOID-based string: 19-character TOID plus a 10-character zero-padded event index, separated by `-`.
- `inSuccessfulContractCall` is deprecated as of recent protocol versions; events in failed transactions are now excluded at the protocol level.
- Events emitted in a failed invocation are discarded and never appear in `getEvents` results.

---

### 1.3 `getLedgerEntries`

**Source:** https://developers.stellar.org/docs/data/apis/rpc/api-reference/methods/getLedgerEntries  
**Accessed:** 2026-07-26 (page also referenced at https://developers.stellar.org/network/soroban-rpc/api-reference/methods/getLedgerEntries as of 2024-04-18)

Returns current on-chain state for one or more ledger entries given their XDR-encoded `LedgerKey`s.

**Request params:**

| Field | Type | Notes |
|---|---|---|
| `keys` | array of strings | Base64-encoded XDR `LedgerKey` structs. |

**Response:**

```json
{
  "jsonrpc": "2.0",
  "id": 8675309,
  "result": {
    "entries": [
      {
        "key": "AAAAB+qfy4Gu...",
        "xdr": "AAAABgAAAAEAAAA...",
        "lastModifiedLedgerSeq": 2552504,
        "liveUntilLedgerSeq": 3088183
      }
    ],
    "latestLedger": 2552990
  }
}
```

Key notes:
- `liveUntilLedgerSeq` is present for `ContractData` and `ContractCode` entries only.
- If an entry is archived (expired), it will not appear in `entries`. The response will contain the key with an empty result.
- Ledgers until expiry: `liveUntilLedgerSeq - latestLedger` (use `latestLedger` from the same response as the baseline for consistency).

---

### 1.4 `getLatestLedger`

**Source:** https://developers.stellar.org/docs/data/apis/rpc/api-reference/methods/getLatestLedger  
**Accessed:** 2026-07-26

No params.

**Response:**

```json
{
  "jsonrpc": "2.0",
  "id": 8675309,
  "result": {
    "id": "0a00a9cf845f7af7cff09c66f8ae6480e9971e6e2c7fa4afd8d6266ee13c987b",
    "protocolVersion": 27,
    "sequence": 3730795,
    "closeTime": "1784671645",
    "headerXdr": "CgCpz4Rf...",
    "metadataXdr": "AAAAAgAAAAAK..."
  }
}
```

Notes: `closeTime` is a Unix timestamp string. `protocolVersion` was 27 on testnet as of the accessed date. `headerXdr` and `metadataXdr` are base64-encoded XDR added in more recent protocol versions.

---

### 1.5 `getTransaction`

**Source:** https://developers.stellar.org/docs/data/apis/rpc/api-reference/methods/getTransaction  
**Accessed:** 2026-07-26

**Request params:**

| Field | Type | Notes |
|---|---|---|
| `hash` | string (required) | 64-character hex transaction hash. |
| `xdrFormat` | string | `"base64"` (default) or `"json"`. |

**Response fields (always present):**

| Field | Type | Notes |
|---|---|---|
| `status` | string | `"SUCCESS"`, `"FAILED"`, or `"NOT_FOUND"`. |
| `latestLedger` | number | Latest ledger known at request time. |
| `latestLedgerCloseTime` | number | Unix timestamp. |
| `oldestLedger` | number | Oldest ledger retained in RPC. |
| `oldestLedgerCloseTime` | number | Unix timestamp. |

**Response fields (present when `status` is `SUCCESS` or `FAILED`):**

| Field | Type | Notes |
|---|---|---|
| `ledger` | number | Ledger where transaction was included. |
| `createdAt` | number | Unix timestamp of inclusion. |
| `applicationOrder` | number | Index among all transactions in the ledger. |
| `feeBump` | boolean | Whether this was a fee-bump transaction. |
| `envelopeXdr` | string | Base64 `TransactionEnvelope`. |
| `resultXdr` | string | Base64 `TransactionResult`. |
| `resultMetaXdr` | string | Base64 `TransactionMeta`. |
| `diagnosticEventsXdr` | array of strings | Base64 `DiagnosticEvent`s; only present when `ENABLE_SOROBAN_DIAGNOSTIC_EVENTS` is enabled on the RPC node. |
| `events.transactionEventsXdr` | array of strings | Base64 `TransactionEvent`s (fees charged/refunded). |
| `events.contractEventsXdr` | array of arrays | Nested arrays of base64 `ContractEvent`s per operation. |

**How to distinguish success from failure:**

The `status` field is the primary discriminator:
- `"SUCCESS"`: invocation ran to completion and state changes were committed.
- `"FAILED"`: the transaction was included in a ledger but the invocation failed (e.g. trap, budget exhaustion, auth failure, contract error). State changes are NOT committed, but the transaction fee IS charged.
- `"NOT_FOUND"`: the transaction is not in the RPC retention window (default: last 24 hours, max 7 days).

For more granular failure information, decode `resultXdr` as `TransactionResult` and inspect the inner `InvokeHostFunctionResult`. The `diagnosticEventsXdr` array (when available) contains host-emitted diagnostic events that include the exact error code and call stack. If diagnostics are not available (most public RPC nodes disable them by default), the error is only recoverable from `resultXdr`.

---

### 1.6 `getNetwork`

**Source:** https://developers.stellar.org/docs/data/apis/rpc/api-reference/methods/getNetwork  
**Accessed:** 2026-07-26 (also confirmed at https://developers.stellar.org/network/soroban-rpc/api-reference/methods/getNetwork)

No params.

**Response:**

```json
{
  "jsonrpc": "2.0",
  "id": 8675309,
  "result": {
    "friendbotUrl": "https://friendbot-testnet.stellar.org/",
    "passphrase": "Test SDF Network ; September 2015",
    "protocolVersion": 20
  }
}
```

Notes: `friendbotUrl` is optional and only present on Testnet/Futurenet. `passphrase` is used to validate transaction signatures; it differs per network. `protocolVersion` reflects the protocol of the latest closed ledger.

---

## 2. Public RPC Endpoints and Rate Limits

**Source:** https://developers.stellar.org/docs/data/apis/rpc/providers  
**Accessed:** 2026-07-26

### SDF-operated public free endpoints

| Network | URL |
|---|---|
| Futurenet | `https://rpc-futurenet.stellar.org` |
| Testnet | `https://soroban-testnet.stellar.org` |

SDF does not operate a public free mainnet RPC endpoint. Mainnet requires a commercial provider or self-hosted node.

### Public free endpoints from third parties (as listed in official docs, July 2026)

| Provider | Network | URL |
|---|---|---|
| Liquify | Futurenet | `https://stellar.liquify.com/api=41EEWAH79Y5OCGI7/futurenet` |
| Liquify | Testnet | `https://stellar.liquify.com/api=41EEWAH79Y5OCGI7/testnet` |
| Liquify | Mainnet | `https://stellar-mainnet.liquify.com/api=41EEWAH79Y5OCGI7/mainnet` |
| Gateway | Testnet | `https://soroban-rpc.testnet.stellar.gateway.fm` |
| Gateway | Mainnet | `https://soroban-rpc.mainnet.stellar.gateway.fm` |
| sorobanrpc.com | Mainnet | `https://mainnet.sorobanrpc.com` |
| Nodies | Testnet | `https://stellar-soroban-testnet-public.nodies.app` |
| Nodies | Mainnet | `https://stellar-soroban-public.nodies.app` |
| OnFinality | Mainnet | `https://stellar.api.onfinality.io/public` |
| Lightsail/Quasar | Mainnet | `https://rpc.lightsail.network/` |
| Lightsail/Quasar | Mainnet (archive) | `https://archive-rpc.lightsail.network/` |
| Ankr | Mainnet (archive) | `https://rpc.ankr.com/stellar_soroban` |

### Rate limits

The official docs do not publish specific numeric rate limits for the SDF testnet endpoint. Third-party community observation is that the SDF testnet endpoint returns HTTP 429 under heavy polling (> ~30 req/min per IP in practice, but this is unverified and subject to change). Commercial providers (Ankr, QuickNode, Alchemy, etc.) publish per-plan limits on their own dashboards.

**Recommendation for Sorolens:** Use the SDF testnet endpoint for development. For production mainnet indexing, use a commercial provider. Upstash Redis-based rate-limit backoff should guard against 429 errors on any public endpoint.

---

## 3. Soroban Storage Durability

**Source:** https://developers.stellar.org/docs/learn/fundamentals/contract-development/storage/state-archival  
**Accessed:** 2026-07-26 (page last updated 2026-03-02)

### Three storage types

| Type | Access in contract | Expiry behavior | Key space | Size limit |
|---|---|---|---|---|
| `Temporary` | `env.storage().temporary()` | Permanently deleted when TTL reaches 0. Cannot be restored. | Separate per-contract key space | Unlimited |
| `Persistent` | `env.storage().persistent()` | Archived (not deleted) when TTL reaches 0. Can be restored. | Separate per-contract key space | Unlimited |
| `Instance` | `env.storage().instance()` | Archived when TTL reaches 0. Shares the same TTL as the contract instance itself. | Stored inside the single contract instance `LedgerEntry` | ~100 KB serialized |

### `liveUntilLedger` / TTL semantics

Each `ContractData` and `ContractCode` entry carries a `liveUntilLedger` field in its `LedgerEntry`. The entry is live as long as `current_ledger <= liveUntilLedger`. Once `current_ledger > liveUntilLedger`, the entry is expired.

**TTL formula (as returned by `getLedgerEntries`):**

```
ledgers_until_expiry = liveUntilLedgerSeq - latestLedger
```

Always use `latestLedger` from the same `getLedgerEntries` response, not a separately fetched ledger, to avoid race conditions.

If `ledgers_until_expiry <= 0`, the entry is expired (archived for Persistent/Instance, deleted for Temporary).

A Stellar ledger closes approximately every 5-6 seconds. At 6 seconds per ledger:
- 17,280 ledgers ~= 1 day
- 120,960 ledgers ~= 7 days

### Minimum and maximum TTL

Both are network parameters set by validator vote.

- Minimum TTL for Persistent/Instance entries at creation or restoration: approximately `current_ledger + 4,095` ledgers. This value is protocol-configurable.
- Maximum TTL: checked against the current ledger at the time of extension (not creation). Current values are visible at https://lab.stellar.org/network-limits.

### Archival and restoration

**Automatic archival:** When `current_ledger > liveUntilLedger`, the network moves the entry out of the live ledger state (BucketList) into off-chain archival storage (ESS, External State Store). For Temporary entries, they are simply deleted.

**Restoration (Protocol 23+):** Starting with Protocol 23 (CAP-0066), archived Persistent and Instance entries are automatically restored when they appear in a transaction's restore list. In practice, `simulateTransaction` detects access to archived entries and populates the restore list; the resulting `InvokeHostFunction` operation then restores them before running. Manual `RestoreFootprintOp` is only needed in edge cases (e.g. when auto-restoration would make the transaction too large).

**Important for Sorolens:** When tracking storage entries, the indexer must check `liveUntilLedgerSeq` on each poll and update the `storage_entries.status` column accordingly. An entry that was live on the previous poll may be archived by the time of the next poll.

---

## 4. Resource Fees and Invocation Results

**Source:** https://developers.stellar.org/docs/learn/fundamentals/fees-resource-limits-metering  
**Accessed:** 2026-07-26

### Fee structure for Soroban transactions

A Soroban transaction carries two fee components:

1. **Inclusion fee (classic fee):** The fee bid for network inclusion; paid to validators regardless of invocation outcome. This is the `fee` field on the `TransactionEnvelope`.

2. **Resource fee (Soroban-specific):** Covers CPU instructions, memory, ledger read/write bytes, event emission bytes, and ledger space rent (TTL extension payments). The resource fee is always charged, but any unused portion is refunded. The refund appears as a `TransactionEvent` of type `fee` with a negative XLM amount in `events.transactionEventsXdr` on the `getTransaction` response.

### How resource consumption is reported

The `resultMetaXdr` field on `getTransaction` decodes to a `TransactionMeta` XDR struct. In `TransactionMetaV3` and `TransactionMetaV4` (Protocol 20+):

- `sorobanMeta.events`: custom events emitted by the contract (present only on success).
- `sorobanMeta.diagnosticEvents`: host diagnostic events including `fn_call`, `fn_return`, and `core_metrics` events. Each `core_metrics` event contains a single resource counter (e.g. `cpu_insn`, `mem_byte`, `read_entry`, `write_entry`, `ledger_read_byte`, `ledger_write_byte`).

The `diagnosticEventsXdr` array on `getTransaction` contains the same diagnostic events in a more directly accessible form, but it requires the RPC node to have `ENABLE_SOROBAN_DIAGNOSTIC_EVENTS=true`. Most public RPC nodes do NOT enable this.

### Distinguishing success from failure in practice

```
status == "SUCCESS"  -> invocation completed, state committed
status == "FAILED"   -> invocation failed, state NOT committed, fee charged
status == "NOT_FOUND" -> outside retention window (default 24h, max 7d)
```

For FAILED transactions, further detail requires decoding `resultXdr` as `TransactionResult`. The inner `invokeHostFunctionResult` will have a result code (e.g. `INVOKE_HOST_FUNCTION_TRAPPED`, `INVOKE_HOST_FUNCTION_RESOURCE_LIMIT_EXCEEDED`, `INVOKE_HOST_FUNCTION_INSUFFICIENT_REFUNDABLE_FEE`).

**For Sorolens invocations table:** Store `status` as a VARCHAR with values `SUCCESS`, `FAILED`, `NOT_FOUND`; store `result_xdr` as TEXT for downstream decoding; store `resource_fee_charged` in stroops (1 XLM = 10,000,000 stroops) extracted from `resultXdr.feeCharged`.

---

## 5. Existing Tools: Gap Analysis

This section is the most important part of this document. The question is whether Sorolens has a genuine gap to fill, or whether existing tooling already covers this.

### 5.1 Stellar Expert (`stellar.expert`)

**What it does well:**
- Transaction history for any contract address, with ledger-close timestamps, operation counts, and fee summaries.
- Contract page at `stellar.expert/explorer/public/contract/<ID>` shows: creation date, WASM hash, contract code validation status (via SEP-based GitHub attestation), network of deployment.
- Protocol history page tracks network-wide Soroban parameter upgrades over time.
- Excellent for classic Stellar: payments, offers, account management, asset analytics.
- The explorer itself noted state inconsistencies following the emergency protocol 24 upgrade in late 2025, with recovery ETA given as December 2025. This history illustrates that Stellar Expert's Soroban support is secondary to its core mission.

**What it does NOT do for Soroban specifically:**
- Does not expose decoded contract storage entries (no TTL dashboard, no storage key/value viewer).
- Does not expose resource consumption metrics per invocation (no CPU, memory, or ledger byte breakdowns).
- Does not expose invocation-level decoded arguments or return values (all contract call data is presented as raw XDR blobs).
- Does not track the expiry status of storage entries over time; no alerting or time-series view of TTL.
- Does not provide a cross-contract activity feed or aggregated event stream for a tracked set of contracts.
- Does not expose the distinction between temporary, persistent, and instance storage.

**Assessment:** Stellar Expert is a strong general blockchain explorer. Its Soroban contract page shows you that a contract exists and links transactions to it, but it is not an observability tool. A developer trying to understand why a contract function failed, what resources it consumed, or which storage entries are close to expiry gets essentially nothing from Stellar Expert.

---

### 5.2 Stellar Lab Contract Explorer (`lab.stellar.org`)

**Source:** https://developers.stellar.org/docs/tools/lab/smart-contracts/contract-explorer  
**Accessed:** 2026-07-26

**What it does well:**
- Shows all current storage entries for a contract in decoded (human-readable) form, sortable and filterable by key, value, durability, TTL, and last-modified ledger.
- Displays contract spec (ABI equivalent), environment metadata, and version history (WASM upgrades).
- Allows invoke-by-form: select a function, fill in typed parameters, simulate or submit.
- Shows build verification status (GitHub attestation via SEP).
- Allows restore of archived entries directly from the UI (generates a `RestoreFootprintOp` transaction).
- Exports storage entries in XDR or JSON.

**What it does NOT do for Soroban specifically:**
- This is a single-contract, point-in-time inspection tool only. It has no time-series or historical view.
- No event history: you cannot see what events a contract emitted over a range of ledgers.
- No invocation history: you cannot see the last N calls to a contract, their arguments, results, or resource consumption.
- No cross-contract or multi-contract views.
- No alerting or monitoring hooks (e.g. notify when a storage entry is within N ledgers of expiry).
- Entirely stateless: every page load fetches live data from the RPC. There is no index, no persistence, no API.
- Requires manual operation: a developer must go to the page, type a contract ID, and press buttons. Nothing is automated or push-based.

**Assessment:** The Lab Contract Explorer is excellent for development-time inspection. It covers current storage state better than anything else in the ecosystem. However, it is not a backend service, it is not queryable by API, and it has no time-series or event-history capabilities. It is a developer scratchpad, not an observability platform.

---

### 5.3 Sunday-Explorer / Stellar-Soroban-Contract-Explorer

**Source:** https://github.com/Sunday-Explorer/Stellar-Soroban-Contract-Explorer  
**Accessed:** 2026-07-26

An open-source project that provides a stateless read-only API (NestJS backend) that decodes storage entries, events, and invocations via XDR decoding for a given contract. Frontend is in a separate repo.

**What it does:** Decoded storage entries (key, value, durability, lastModifiedLedger), decoded events, decoded function invocations (function name, args, result).

**Limitations:**
- Stateless: no database, no persistence, no time-series. Every request re-fetches from the RPC.
- No TTL tracking, no expiry alerting.
- No resource fee breakdowns.
- Limited by the RPC retention window (24h for events).
- Not production-deployed to a public URL in active use.

**Assessment:** This project confirms that a gap exists in the ecosystem but does not fully close it. It does not index data, so it cannot answer "how many times was this function called last week" or "show me all storage entries that expire within 5 days."

---

### 5.4 ShippedLabs / Soroban-Contract-Explorer

**Source:** https://github.com/RaymondAbiola/Soroban-Contract-Explorer  
**Accessed:** 2026-07-26

A no-code web UI for calling contract functions by form. Described as "the closest thing to an Etherscan Read/Write tab for Soroban." Testnet only. Stateless, no indexing.

**Assessment:** Not observability tooling. Only covers contract invocation, not monitoring or event/storage history.

---

### 5.5 Solarkraft

**Source:** https://github.com/freespek/solarkraft  
**Accessed:** 2026-07-26

A runtime monitoring tool powered by TLA+ formal specifications. Inspects invocation history against a property specification and raises alerts when the contract deviates from expected behavior.

**What it does:** Formal verification / property monitoring post-deployment, targeted at auditors and security researchers.

**What it does NOT do:** Does not expose general-purpose observability data (events, storage, resource fees) to developers. Not a dashboard or API. Requires TLA+ knowledge.

**Assessment:** Niche and complementary to Sorolens, not a substitute.

---

### 5.6 Gap Analysis Summary

**Honest verdict: The gap is real but smaller than it might appear if you only know Etherscan.**

The Stellar Lab Contract Explorer covers current storage state well. But no tool in the ecosystem provides all of the following together:

- A persistent indexed database of events and invocations beyond the 24-hour RPC retention window.
- A time-series view of resource consumption per contract (CPU, memory, ledger bytes, fee charged).
- A TTL health dashboard showing which storage entries are approaching expiry.
- A queryable REST API that lets a dApp backend or CI pipeline fetch this data programmatically.
- A multi-contract activity feed.

If Sorolens ships only as another single-contract read-through decoder, it is not differentiated enough. The value comes from three things the existing tools do not have:

1. **Persistence.** Indexed data in Postgres that survives RPC rotation.
2. **Resource metrics.** Per-invocation CPU, memory, and fee breakdowns queryable over time.
3. **TTL health.** A view that shows developers which of their storage entries are at risk of expiry.

The CLI is also a real gap: there is no `stellar-rpc`-level equivalent of `cast call` or `foundry trace` that gives a developer a quick per-invocation resource breakdown from the terminal.

---

## 6. Sources

| URL | Purpose | Accessed |
|---|---|---|
| https://developers.stellar.org/docs/data/apis/rpc/api-reference/methods/getEvents | getEvents method reference | 2026-07-26 |
| https://developers.stellar.org/docs/data/apis/rpc/api-reference/methods/getTransaction | getTransaction method reference | 2026-07-26 |
| https://developers.stellar.org/docs/data/apis/rpc/api-reference/methods/getLatestLedger | getLatestLedger method reference | 2026-07-26 |
| https://developers.stellar.org/docs/data/apis/rpc/api-reference/methods/getNetwork | getNetwork method reference | 2026-07-26 |
| https://developers.stellar.org/network/soroban-rpc/api-reference/methods/getLedgerEntries | getLedgerEntries with example response | 2026-07-26 |
| https://developers.stellar.org/docs/learn/fundamentals/contract-development/storage/state-archival | State archival, TTL, Persistent/Temporary/Instance | 2026-07-26 |
| https://developers.stellar.org/docs/learn/fundamentals/fees-resource-limits-metering | Resource fees and metering | 2026-07-26 |
| https://developers.stellar.org/docs/data/apis/rpc/providers | Public RPC endpoints and providers | 2026-07-26 |
| https://developers.stellar.org/docs/tools/lab/smart-contracts/contract-explorer | Stellar Lab Contract Explorer features | 2026-07-26 |
| https://stellar.org/blog/foundation-news/state-archival-issue-post-mortem | State archival post-mortem (Oct 2025 incident) | 2026-07-26 |
| https://github.com/Sunday-Explorer/Stellar-Soroban-Contract-Explorer | Third-party explorer project | 2026-07-26 |
| https://github.com/freespek/solarkraft | Solarkraft runtime monitoring | 2026-07-26 |
