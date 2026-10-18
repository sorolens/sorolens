-- ============================================================
-- 000037_api_key_expires_at
--
-- Optional expiry on API keys. A key stops authenticating requests
-- once `expires_at` passes; revoked keys stay unusable regardless of
-- expiry. NULL means the key never expires. Key creation, rotation and
-- revocation are all audited via the existing audit_events table
-- (000026_audit_events) and the router's Audit middleware.
-- ============================================================

ALTER TABLE api_keys
    ADD COLUMN expires_at TIMESTAMPTZ;

CREATE INDEX idx_api_keys_expires_at ON api_keys (expires_at) WHERE expires_at IS NOT NULL;
