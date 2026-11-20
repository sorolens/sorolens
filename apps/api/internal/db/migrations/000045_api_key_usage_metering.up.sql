-- 000045_api_key_usage_metering
-- Add monthly quota support and daily endpoint-group usage tracking for API keys (issue #792).

ALTER TABLE api_keys ADD COLUMN monthly_quota INT;

CREATE TABLE api_key_usage (
    key_id TEXT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    endpoint_group TEXT NOT NULL,
    request_count BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (key_id, date, endpoint_group)
);

CREATE INDEX idx_api_key_usage_key_date ON api_key_usage (key_id, date);
