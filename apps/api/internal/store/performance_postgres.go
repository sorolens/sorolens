package store

import (
	"context"
	"fmt"
	"time"
)

// ComputeAndStoreBaselines computes the 7-day average for each (contract, function)
// up to the snapshotDate, and stores it in performance_baselines idempotently.
func (p *postgresStore) ComputeAndStoreBaselines(ctx context.Context, snapshotDate time.Time) error {
	query := `
		INSERT INTO performance_baselines (contract_id, function_name, avg_cpu, avg_mem, avg_fee, snapshot_date)
		SELECT 
			contract_id,
			function_name,
			COALESCE(AVG(cpu_insn), 0)::BIGINT as avg_cpu,
			COALESCE(AVG(mem_byte), 0)::BIGINT as avg_mem,
			COALESCE(AVG(resource_fee_charged), 0)::BIGINT as avg_fee,
			$1::DATE
		FROM invocations
		WHERE ledger_closed_at >= $1::DATE - INTERVAL '7 days'
		  AND ledger_closed_at < $1::DATE
		  AND function_name IS NOT NULL
		  AND status = 'SUCCESS'
		GROUP BY contract_id, function_name
		ON CONFLICT (contract_id, function_name, snapshot_date) DO UPDATE
		SET 
			avg_cpu = EXCLUDED.avg_cpu,
			avg_mem = EXCLUDED.avg_mem,
			avg_fee = EXCLUDED.avg_fee;
	`

	_, err := p.pool.Exec(ctx, query, snapshotDate)
	if err != nil {
		return fmt.Errorf("failed to compute baselines: %w", err)
	}
	return nil
}

// CheckAndEmitRegressions compares current 7d averages against the previous baseline
// and emits a 'Warning' alert to contract_alerts if any metric increased by > 25%.
func (p *postgresStore) CheckAndEmitRegressions(ctx context.Context, snapshotDate time.Time) (int, error) {
	// 1. Get the current 7d averages
	// 2. Join with the most recent baseline before current snapshotDate
	// 3. If current > 1.25 * baseline, insert into contract_alerts.

	query := `
		WITH current_avgs AS (
			SELECT 
				contract_id,
				function_name,
				COALESCE(AVG(cpu_insn), 0)::BIGINT as curr_cpu,
				COALESCE(AVG(mem_byte), 0)::BIGINT as curr_mem,
				COALESCE(AVG(resource_fee_charged), 0)::BIGINT as curr_fee
			FROM invocations
			WHERE ledger_closed_at >= $1::DATE - INTERVAL '7 days'
			  AND ledger_closed_at < $1::DATE
			  AND function_name IS NOT NULL
			  AND status = 'SUCCESS'
			GROUP BY contract_id, function_name
		),
		latest_baselines AS (
			SELECT DISTINCT ON (contract_id, function_name)
				contract_id, function_name, avg_cpu, avg_mem, avg_fee
			FROM performance_baselines
			WHERE snapshot_date < $1::DATE
			ORDER BY contract_id, function_name, snapshot_date DESC
		),
		regressions AS (
			SELECT
				c.contract_id,
				c.function_name,
				c.curr_cpu, b.avg_cpu,
				c.curr_mem, b.avg_mem,
				c.curr_fee, b.avg_fee
			FROM current_avgs c
			JOIN latest_baselines b ON c.contract_id = b.contract_id AND c.function_name = b.function_name
			JOIN monitored_contracts m ON c.contract_id = m.contract_id
			WHERE 
				(b.avg_cpu > 0 AND c.curr_cpu > b.avg_cpu * 1.25) OR
				(b.avg_mem > 0 AND c.curr_mem > b.avg_mem * 1.25) OR
				(b.avg_fee > 0 AND c.curr_fee > b.avg_fee * 1.25)
		)
		INSERT INTO contract_alerts (contract_id, severity, message, ledger, tx_hash, timestamp)
		SELECT 
			contract_id,
			'Warning',
			'Performance regression detected in function ' || function_name || '. CPU: ' || curr_cpu || ' (baseline ' || avg_cpu || '), Mem: ' || curr_mem || ' (baseline ' || avg_mem || '), Fee: ' || curr_fee || ' (baseline ' || avg_fee || ')',
			0, -- ledger is 0 for system-generated alerts
			'system-performance-regression-' || $1::DATE || '-' || function_name, -- pseudo tx_hash to satisfy uniqueness constraint
			$1
		FROM regressions
		ON CONFLICT (tx_hash, contract_id) DO NOTHING;
	`

	cmd, err := p.pool.Exec(ctx, query, snapshotDate)
	if err != nil {
		return 0, fmt.Errorf("failed to check regressions: %w", err)
	}

	return int(cmd.RowsAffected()), nil
}
