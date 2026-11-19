CREATE TABLE contract_health_score_history (
    contract_id TEXT NOT NULL
        REFERENCES contract_health_scores (contract_id) ON DELETE CASCADE,
    bucket_at TIMESTAMPTZ NOT NULL,
    computed_at TIMESTAMPTZ NOT NULL,
    score INTEGER NOT NULL CHECK (score BETWEEN 0 AND 100),
    PRIMARY KEY (contract_id, bucket_at)
);

CREATE FUNCTION record_contract_health_score_history()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO contract_health_score_history (
        contract_id, bucket_at, computed_at, score
    )
    VALUES (
        NEW.contract_id,
        date_trunc('hour', NEW.computed_at, 'UTC'),
        NEW.computed_at,
        NEW.score
    )
    ON CONFLICT (contract_id, bucket_at) DO UPDATE SET
        computed_at = EXCLUDED.computed_at,
        score = EXCLUDED.score
    WHERE contract_health_score_history.computed_at <= EXCLUDED.computed_at;

    RETURN NEW;
END;
$$;

CREATE TRIGGER record_contract_health_score_history
AFTER INSERT OR UPDATE ON contract_health_scores
FOR EACH ROW
EXECUTE FUNCTION record_contract_health_score_history();

INSERT INTO contract_health_score_history (
    contract_id, bucket_at, computed_at, score
)
SELECT contract_id, date_trunc('hour', computed_at, 'UTC'), computed_at, score
FROM contract_health_scores
ON CONFLICT (contract_id, bucket_at) DO NOTHING;