-- Bind an API key to a user so the key can carry a verified identity
-- (issue #518). Unbound keys keep authenticating their scopes but cannot act
-- as a user or reach user-scoped data. ON DELETE SET NULL keeps the key
-- usable for machine-to-machine scope checks when the user row is removed.
ALTER TABLE api_keys
    ADD COLUMN user_id TEXT REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX idx_api_keys_user_id ON api_keys (user_id) WHERE user_id IS NOT NULL;
