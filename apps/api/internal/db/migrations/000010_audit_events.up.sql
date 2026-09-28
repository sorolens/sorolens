-- ============================================================
-- 000010_audit_events
--
-- Structured audit trail for state-changing API calls (issue #122).
-- The router middleware writes one row per non-GET request after the
-- handler returns, including failed requests (status carries the
-- response code). Only a SHA-256 hash of the request body is kept,
-- never the raw body.
--
-- Queried by GET /api/v1/admin/audit?since=&limit= (admin-only),
-- newest first (by id); `since` filters on the `at` index.
-- ============================================================

CREATE TABLE audit_events (
    id                BIGSERIAL   PRIMARY KEY,
    actor             TEXT        NOT NULL DEFAULT '',
    action            TEXT        NOT NULL,
    resource_type     TEXT        NOT NULL DEFAULT '',
    resource_id       TEXT        NOT NULL DEFAULT '',
    ip                TEXT        NOT NULL DEFAULT '',
    user_agent        TEXT        NOT NULL DEFAULT '',
    request_body_hash TEXT        NOT NULL DEFAULT '',
    status            INTEGER     NOT NULL,
    at                TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_events_at ON audit_events (at);
