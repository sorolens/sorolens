CREATE TABLE performance_baselines (
    id            BIGSERIAL PRIMARY KEY,
    contract_id   TEXT      NOT NULL REFERENCES contracts (id) ON DELETE CASCADE,
    function_name TEXT      NOT NULL,
    avg_cpu       BIGINT    NOT NULL,
    avg_mem       BIGINT    NOT NULL,
    avg_fee       BIGINT    NOT NULL,
    snapshot_date DATE      NOT NULL,
    UNIQUE (contract_id, function_name, snapshot_date)
);

CREATE INDEX idx_performance_baselines_contract_func
    ON performance_baselines (contract_id, function_name, snapshot_date DESC);
