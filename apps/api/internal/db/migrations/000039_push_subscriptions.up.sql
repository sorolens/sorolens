-- ============================================================
-- 000039_push_subscriptions
--
-- Browser Web Push subscriptions (issue #592). Previously these lived only
-- in an in-memory Map in apps/web/app/api/push/subscribe/route.ts and were
-- lost on every cold start; the API now owns persistence.
--
-- endpoint          the push-service URL the browser reports. It is unique:
--                   re-subscribing with the same endpoint updates the row in
--                   place (the browser reuses endpoints across sessions),
--                   which is what makes POST idempotent.
-- p256dh_key/auth_key the subscriber's ECDH public key and auth secret
--                   (RFC 8291). These are credentials for the push channel —
--                   never log them and never return auth_key from anonymous
--                   surfaces.
-- user_id           optional owner binding; subscriptions made by a
--                   signed-in dashboard user are attributable, anonymous
--                   browser subscriptions leave it NULL.
-- filters           JSONB subscription preferences (contract ids,
--                   severities) so alert fan-out can filter server-side.
-- ============================================================

CREATE TABLE push_subscriptions (
    id          TEXT        PRIMARY KEY,
    endpoint    TEXT        NOT NULL UNIQUE,
    p256dh_key  TEXT        NOT NULL,
    auth_key    TEXT        NOT NULL,
    user_id     TEXT,
    filters     JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Push-service endpoints are always https; enforcing it at the row level
    -- means a store-level bug cannot turn the table into an http exfiltrator.
    CONSTRAINT push_subscriptions_endpoint_https
        CHECK (endpoint LIKE 'https://%'),
    CONSTRAINT push_subscriptions_endpoint_len
        CHECK (char_length(endpoint) <= 2048)
);

-- Fan-out for one user's devices.
CREATE INDEX idx_push_subscriptions_user
    ON push_subscriptions (user_id)
    WHERE user_id IS NOT NULL;
