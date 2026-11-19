DROP TRIGGER IF EXISTS record_contract_health_score_history
    ON contract_health_scores;
DROP FUNCTION IF EXISTS record_contract_health_score_history();
DROP TABLE IF EXISTS contract_health_score_history;