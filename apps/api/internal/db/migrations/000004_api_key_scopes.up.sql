-- ============================================================
-- 000004_api_key_scopes
--
-- User API keys with scoped permissions. Keys are stored as
-- SHA-256 hashes; the plaintext token is shown to the caller once at
-- creation time and never persisted.
--
-- Scopes are a text[] so a key can hold any combination of:
--   read:contracts, write:contracts, read:watchdog, admin:*
-- An empty array means "no scopes" (a valid but useless key).
-- ============================================================

CREATE TABLE api_keys (
    id          TEXT        PRIMARY KEY,
    name        TEXT        NOT NULL,
    key_prefix  TEXT        NOT NULL,                 -- first 8 chars, for display
    key_hash    TEXT        NOT NULL UNIQUE,          -- sha256 hex of the plaintext token
    scopes      TEXT[]      NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ,
    revoked_at  TIMESTAMPTZ
);

-- Authentication looks keys up by hash on every request.
CREATE INDEX idx_api_keys_hash ON api_keys (key_hash) WHERE revoked_at IS NULL;
