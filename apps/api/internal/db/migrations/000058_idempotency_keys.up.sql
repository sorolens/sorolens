-- Issue #784: Idempotency-Key support for mutating endpoints.
--
-- idempotency_keys stores the request fingerprint (SHA-256 of the body) and
-- the first response seen for a given (api_key_id, idempotency_key) pair.
-- A retry with the same key and same body replays the stored response; a retry
-- with a different body returns 422 Unprocessable Entity.
--
-- Keys expire after 24 hours (the documented retention window).  Cleanup is
-- done by a periodic DELETE WHERE expires_at < NOW(), not by a database TTL.
CREATE TABLE IF NOT EXISTS idempotency_keys (
    api_key_id       TEXT        NOT NULL,
    idempotency_key  TEXT        NOT NULL,
    -- SHA-256 hex digest of the raw request body.
    body_fingerprint TEXT        NOT NULL,
    -- Stored response (status code, headers as JSONB, body as text).
    response_status  INT         NOT NULL,
    response_headers JSONB       NOT NULL DEFAULT '{}',
    response_body    TEXT        NOT NULL DEFAULT '',
    -- in_flight prevents two concurrent identical requests from both executing.
    -- The first request claims it (TRUE), the second waits.
    in_flight        BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at       TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '24 hours',
    PRIMARY KEY (api_key_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_idempotency_keys_expires_at
    ON idempotency_keys (expires_at);
