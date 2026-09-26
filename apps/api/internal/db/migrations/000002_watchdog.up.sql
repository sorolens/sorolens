-- ============================================================
-- monitored_contracts
-- One row per contract registered with the Sorolens Watchdog
-- on-chain contract. Materialised from HealthCheckEvent,
-- ContractRegistered, and ContractDeregistered emitted by the
-- watchdog contract.
-- ============================================================
CREATE TABLE monitored_contracts (
    contract_id       TEXT        PRIMARY KEY,
    name              TEXT        NOT NULL,
    owner             TEXT        NOT NULL,
    status            TEXT        NOT NULL DEFAULT 'Healthy',
    last_check        TIMESTAMPTZ,
    check_interval    BIGINT      NOT NULL DEFAULT 0,
    registered_at     TIMESTAMPTZ NOT NULL,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Dashboard filter: show only currently unhealthy contracts.
CREATE INDEX idx_monitored_contracts_status ON monitored_contracts (status);

-- ============================================================
-- health_checks
-- Every HealthCheckEvent pushed by a monitored contract.
-- ============================================================
CREATE TABLE health_checks (
    id            BIGSERIAL   PRIMARY KEY,
    contract_id   TEXT        NOT NULL REFERENCES monitored_contracts (contract_id) ON DELETE CASCADE,
    status        TEXT        NOT NULL,
    metadata      TEXT,
    ledger        BIGINT      NOT NULL,
    tx_hash       TEXT        NOT NULL,
    timestamp     TIMESTAMPTZ NOT NULL,
    inserted_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Health timeline for one contract, newest first.
CREATE INDEX idx_health_checks_contract_ts
    ON health_checks (contract_id, timestamp DESC);

-- Dedupe on-chain replays across indexer runs.
CREATE UNIQUE INDEX ux_health_checks_tx_contract
    ON health_checks (tx_hash, contract_id);

-- ============================================================
-- contract_alerts
-- Alerts emitted for monitored contracts.
-- ============================================================
CREATE TABLE contract_alerts (
    id            BIGSERIAL   PRIMARY KEY,
    contract_id   TEXT        NOT NULL REFERENCES monitored_contracts (contract_id) ON DELETE CASCADE,
    severity      TEXT        NOT NULL,
    message       TEXT        NOT NULL,
    ledger        BIGINT      NOT NULL,
    tx_hash       TEXT        NOT NULL,
    timestamp     TIMESTAMPTZ NOT NULL,
    inserted_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_contract_alerts_contract_ts
    ON contract_alerts (contract_id, timestamp DESC);

CREATE INDEX idx_contract_alerts_severity
    ON contract_alerts (severity, timestamp DESC);

CREATE UNIQUE INDEX ux_contract_alerts_tx_contract
    ON contract_alerts (tx_hash, contract_id);
