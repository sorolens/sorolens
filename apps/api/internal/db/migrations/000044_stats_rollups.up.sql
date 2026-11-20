-- ============================================================
-- 000044_stats_rollups
--
-- Pre-aggregated hourly and daily rollup tables for stats endpoints (issue #788).
-- The indexer writes to these incrementally after each committed window;
-- stats queries read from rollups when the window aligns, falling back to
-- raw table scans for sub-hour windows.
-- ============================================================

CREATE TABLE events_rollup_hourly (
    network      TEXT        NOT NULL,
    contract_id  TEXT        NOT NULL DEFAULT '',
    hour         TIMESTAMPTZ NOT NULL,
    event_count  BIGINT      NOT NULL DEFAULT 0,
    invoke_count BIGINT      NOT NULL DEFAULT 0,
    fail_count   BIGINT      NOT NULL DEFAULT 0,
    PRIMARY KEY (network, contract_id, hour)
);

-- Network-wide aggregate rows have contract_id = ''.
CREATE INDEX idx_rollup_hourly_network ON events_rollup_hourly (network, hour DESC)
    WHERE contract_id = '';

-- Per-contract queries ordered by time.
CREATE INDEX idx_rollup_hourly_contract ON events_rollup_hourly (contract_id, hour DESC)
    WHERE contract_id != '';

CREATE TABLE events_rollup_daily (
    network      TEXT NOT NULL,
    contract_id  TEXT NOT NULL DEFAULT '',
    day          DATE NOT NULL,
    event_count  BIGINT NOT NULL DEFAULT 0,
    invoke_count BIGINT NOT NULL DEFAULT 0,
    fail_count   BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (network, contract_id, day)
);

CREATE INDEX idx_rollup_daily_network ON events_rollup_daily (network, day DESC)
    WHERE contract_id = '';

CREATE INDEX idx_rollup_daily_contract ON events_rollup_daily (contract_id, day DESC)
    WHERE contract_id != '';

-- Backfill hourly rollups from existing events data.
INSERT INTO events_rollup_hourly (network, contract_id, hour, event_count)
SELECT network, contract_id, date_trunc('hour', ledger_closed_at), COUNT(*)
FROM events
GROUP BY network, contract_id, date_trunc('hour', ledger_closed_at)
ON CONFLICT DO NOTHING;

-- Backfill daily rollups from existing events data.
INSERT INTO events_rollup_daily (network, contract_id, day, event_count)
SELECT network, contract_id, date_trunc('day', ledger_closed_at)::date, COUNT(*)
FROM events
GROUP BY network, contract_id, date_trunc('day', ledger_closed_at)::date
ON CONFLICT DO NOTHING;
