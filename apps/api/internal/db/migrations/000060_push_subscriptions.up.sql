-- Persistent Web Push subscriptions (issue #592).
--
-- Previously the dashboard kept these in an in-memory Map, so every cold start
-- forgot every device. The endpoint is unique: re-subscribing the same browser
-- updates its keys, owner and filters instead of adding a duplicate row.
CREATE TABLE push_subscriptions (
    id          TEXT        PRIMARY KEY,
    endpoint    TEXT        NOT NULL UNIQUE,
    auth_key    TEXT        NOT NULL,
    p256dh_key  TEXT        NOT NULL,
    user_id     TEXT        NOT NULL,
    filters     JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE push_subscriptions IS
    'Browser Web Push endpoints, owned by a workspace user (issue #592).';
COMMENT ON COLUMN push_subscriptions.filters IS
    'Optional delivery filters: severity, contract_id and network.';

CREATE INDEX push_subscriptions_user_id_idx ON push_subscriptions (user_id);
