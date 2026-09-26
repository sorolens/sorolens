-- ============================================================
-- 000008_failed_events
--
-- Dead-letter queue (DLQ) for events that fail processing after
-- retries (issue #202). One bad event must not block the indexer
-- pipeline: after 3 attempts the event is parked here with the
-- last error so operators can inspect and requeue it.
-- ============================================================

CREATE TABLE failed_events (
    id               BIGSERIAL   PRIMARY KEY,
    event_id         TEXT        NOT NULL,
    contract_id      TEXT        NOT NULL,
    network          TEXT        NOT NULL DEFAULT '',
    event_payload    JSONB       NOT NULL,
    error_message    TEXT        NOT NULL,
    attempts         INTEGER     NOT NULL DEFAULT 3,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (event_id)
);

-- Newest failures first for the DLQ list endpoint.
CREATE INDEX idx_failed_events_created_at ON failed_events (created_at DESC);

-- Filter DLQ by contract when operators triage a single contract.
CREATE INDEX idx_failed_events_contract_id ON failed_events (contract_id);
