-- ============================================================
-- 000009_call_edges
--
-- Cross-contract call graph (issue: "cross-contract call graph
-- tracing in the indexer").
--
-- One row per parent -> child invocation edge inside a single
-- transaction, materialised by the indexer from the Soroban host
-- diagnostic event stream (fn_call / fn_return / core_metrics).
--
-- The ROOT invocation is deliberately NOT duplicated here: it
-- already lives in the `invocations` table (tx_hash PRIMARY KEY),
-- exactly as the issue's "reuse the existing invocations table
-- for the root row" guidance asks. call_edges describes the tree
-- *below* that root.
--
-- span ids are deterministic call-path strings built by the
-- indexer ("0", "0.0", "0.1", "0.0.0", ...), not random ids, so
-- re-indexing a transaction is idempotent and the whole tree can
-- be rebuilt from a single ordered read of (tx_hash, child_span_id).
-- ============================================================

CREATE TABLE call_edges (
    tx_hash            TEXT        NOT NULL,
    -- Span id of the caller. "0" is the transaction's root invocation.
    parent_span_id     TEXT        NOT NULL,
    -- Span id of the callee; the path encodes the call depth.
    child_span_id      TEXT        NOT NULL,
    -- Contract that was called. NULL when the host frame has no callee
    -- contract (e.g. a host function invoked directly).
    callee_contract_id TEXT,
    function_name      TEXT,
    -- CPU instructions attributed to the frame, from core_metrics
    -- diagnostic events when the RPC node emits them, else 0.
    cpu                BIGINT      NOT NULL DEFAULT 0,
    -- Memory bytes attributed to the frame, same caveat as cpu.
    mem                BIGINT      NOT NULL DEFAULT 0,
    -- Share of the root invocation's resource fee, in stroops,
    -- distributed across top-level edges by the indexer.
    fee_share          BIGINT      NOT NULL DEFAULT 0,
    -- Call depth below the root: 1 for a direct child of the root.
    depth              INTEGER     NOT NULL DEFAULT 1,
    network            TEXT        NOT NULL DEFAULT 'testnet',
    ledger             BIGINT,
    ledger_closed_at   TIMESTAMPTZ,
    inserted_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (tx_hash, child_span_id)
);

-- Primary query pattern: fetch every edge of one transaction for the
-- /invocations/{tx_hash}/trace endpoint, parent before child.
CREATE INDEX idx_call_edges_tx_hash ON call_edges (tx_hash, child_span_id);

-- Reverse view: "which contracts call this one?" - feeds the
-- cross-contract graph on the contract detail page.
CREATE INDEX idx_call_edges_callee ON call_edges (callee_contract_id);

-- Backfill driver: find transactions whose call graph has not been
-- materialised yet (call_edges LEFT JOIN invocations).
CREATE INDEX idx_call_edges_ledger ON call_edges (ledger DESC);
