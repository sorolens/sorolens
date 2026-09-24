-- ============================================================
-- 000007_contract_health_scores
--
-- Cached composite 0-100 health score per contract (issue #137).
-- The indexer recomputes the score every poll cycle from four
-- normalized inputs and upserts it here so the API and dashboard
-- read a single cheap row instead of recomputing aggregates.
--
-- component_* columns carry the normalized 0-100 value of each
-- input (uptime from the watchdog, error rate from failed
-- invocations, performance from the CPU/fee trend, storage TTL
-- headroom) so the UI can render a breakdown without extra
-- queries.
-- ============================================================

CREATE TABLE contract_health_scores (
    contract_id           TEXT        PRIMARY KEY,
    score                 INTEGER     NOT NULL CHECK (score BETWEEN 0 AND 100),
    component_uptime      INTEGER     NOT NULL DEFAULT 0 CHECK (component_uptime BETWEEN 0 AND 100),
    component_error_rate  INTEGER     NOT NULL DEFAULT 0 CHECK (component_error_rate BETWEEN 0 AND 100),
    component_performance INTEGER     NOT NULL DEFAULT 0 CHECK (component_performance BETWEEN 0 AND 100),
    component_storage_ttl INTEGER    NOT NULL DEFAULT 0 CHECK (component_storage_ttl BETWEEN 0 AND 100),
    computed_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_contract_health_scores_updated
    ON contract_health_scores (computed_at DESC);