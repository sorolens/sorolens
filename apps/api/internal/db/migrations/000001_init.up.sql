-- ============================================================
-- contracts
-- Tracks each contract being observed.
-- ============================================================
CREATE TABLE contracts (
    id                   TEXT        PRIMARY KEY,
    network              TEXT        NOT NULL,
    label                TEXT,
    wasm_hash            TEXT,
    created_at_ledger    BIGINT,
    backfill_complete_at TIMESTAMPTZ,
    -- pending | backfilling | active | paused | error
    status               TEXT        NOT NULL DEFAULT 'pending',
    added_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Dashboard lists contracts per network; network filter is the most common
-- WHERE clause on the contracts table.
CREATE INDEX idx_contracts_network ON contracts (network);

-- ============================================================
-- events
-- One row per emitted contract event.
-- ============================================================
CREATE TABLE events (
    id                 TEXT        PRIMARY KEY,
    contract_id        TEXT        NOT NULL REFERENCES contracts (id),
    ledger             BIGINT      NOT NULL,
    ledger_closed_at   TIMESTAMPTZ NOT NULL,
    tx_hash            TEXT        NOT NULL,
    type               TEXT        NOT NULL,
    topic_xdr          JSONB       NOT NULL,
    value_xdr          TEXT        NOT NULL,
    topic_decoded      JSONB,
    value_decoded      JSONB,
    in_successful_call BOOLEAN     NOT NULL DEFAULT TRUE,
    inserted_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The most common dashboard query: "show me recent events for contract X."
-- Composite index with ledger DESC avoids a sort step.
CREATE INDEX idx_events_contract_ledger ON events (contract_id, ledger DESC);

-- Supports the invocation-detail page which shows all events emitted in a
-- given transaction.
CREATE INDEX idx_events_tx_hash ON events (tx_hash);

-- Time-range filtering on the events feed.
CREATE INDEX idx_events_ledger_closed_at ON events (ledger_closed_at DESC);

-- ============================================================
-- invocations
-- One row per transaction that invoked (or attempted to invoke)
-- a tracked contract.
-- ============================================================
CREATE TABLE invocations (
    tx_hash              TEXT        PRIMARY KEY,
    contract_id          TEXT        NOT NULL REFERENCES contracts (id),
    ledger               BIGINT      NOT NULL,
    ledger_closed_at     TIMESTAMPTZ NOT NULL,
    -- SUCCESS | FAILED | NOT_FOUND
    status               TEXT        NOT NULL,
    function_name        TEXT,
    args_decoded         JSONB,
    result_decoded       JSONB,
    result_xdr           TEXT,
    resource_fee_charged BIGINT,
    cpu_insn             BIGINT,
    mem_byte             BIGINT,
    ledger_read_byte     BIGINT,
    ledger_write_byte    BIGINT,
    application_order    INTEGER,
    inserted_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Primary query pattern: all invocations for a contract, newest first.
CREATE INDEX idx_invocations_contract_ledger ON invocations (contract_id, ledger DESC);

-- Lookup by ledger close time for time-range dashboard queries.
CREATE INDEX idx_invocations_ledger_closed_at ON invocations (ledger_closed_at DESC);

-- Supports the "show only failures" filter on the invocations list.
CREATE INDEX idx_invocations_status ON invocations (contract_id, status);

-- ============================================================
-- storage_entries
-- Snapshot of current contract storage state.
-- Updated on every indexer run for all active contracts.
-- ============================================================
CREATE TABLE storage_entries (
    id                   BIGSERIAL   PRIMARY KEY,
    contract_id          TEXT        NOT NULL REFERENCES contracts (id),
    key_xdr              TEXT        NOT NULL,
    key_decoded          JSONB,
    value_xdr            TEXT,
    value_decoded        JSONB,
    -- temporary | persistent | instance
    durability           TEXT        NOT NULL,
    live_until_ledger    BIGINT,
    last_modified_ledger BIGINT,
    -- live | archived | deleted
    status               TEXT        NOT NULL DEFAULT 'live',
    last_seen_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (contract_id, key_xdr)
);

-- The TTL health view needs to order by live_until_ledger ASC for a given
-- contract. Partial index on status = 'live' avoids scanning archived rows.
CREATE INDEX idx_storage_live_until ON storage_entries (contract_id, live_until_ledger ASC)
    WHERE status = 'live';

-- Supports filtering the storage view by entry type.
CREATE INDEX idx_storage_durability ON storage_entries (contract_id, durability);

-- ============================================================
-- sync_state
-- One row per tracked contract, tracking indexer cursor position.
-- ============================================================
CREATE TABLE sync_state (
    contract_id   TEXT        PRIMARY KEY REFERENCES contracts (id),
    last_ledger   BIGINT      NOT NULL DEFAULT 0,
    last_run_at   TIMESTAMPTZ,
    error_message TEXT,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
