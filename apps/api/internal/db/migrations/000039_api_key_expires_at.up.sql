-- Add optional expires_at on api_keys for key expiration.
ALTER TABLE api_keys ADD COLUMN expires_at TIMESTAMPTZ;
CREATE INDEX idx_api_keys_expires_at ON api_keys (expires_at) WHERE expires_at IS NOT NULL;
