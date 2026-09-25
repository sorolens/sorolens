CREATE TABLE alert_subscriptions (
    id TEXT PRIMARY KEY,
    contract_id TEXT NOT NULL REFERENCES monitored_contracts(contract_id) ON DELETE CASCADE,
    webhook_url TEXT NOT NULL,
    severity_filter TEXT NOT NULL DEFAULT 'Critical',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_alert_subscriptions_contract ON alert_subscriptions(contract_id);
