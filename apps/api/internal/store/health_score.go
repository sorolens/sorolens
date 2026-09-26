package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// storageHealthyHorizonLedgers is how far ahead a live storage entry's
// live_until_ledger must be to count as "healthy" TTL headroom. Set to
// roughly one day of ledgers at ~5s each so expiring entries surface early.
const storageHealthyHorizonLedgers int64 = 17_280

// HealthScoreStore is the read/write surface for cached composite health
// scores (issue #137). Kept as its own interface so the API and the indexer
// adapter can be wired independently, mirroring WatchdogStore and
// ContractUpgradeStore.
type HealthScoreStore interface {
	// ContractHealthInputs aggregates the raw signals that feed the composite
	// health score for a contract. It never errors on empty data: a contract
	// with no watchdog registration, invocations, activity, or storage still
	// yields a zero-value inputs struct.
	ContractHealthInputs(ctx context.Context, contractID string) (HealthScoreInputs, error)
	// UpsertContractHealthScore caches a computed score, replacing any
	// previously cached value for the contract.
	UpsertContractHealthScore(ctx context.Context, h ContractHealthScore) error
	// GetContractHealthScore returns the most recently cached score for a
	// contract, or ErrNotFound when the indexer has not computed one yet.
	GetContractHealthScore(ctx context.Context, contractID string) (ContractHealthScore, error)
}

// ---- postgres implementation ----------------------------------------------

func (s *postgresStore) ContractHealthInputs(ctx context.Context, contractID string) (HealthScoreInputs, error) {
	var in HealthScoreInputs

	// Watchdog history: last 100 health checks and the contract's live status.
	row := s.pool.QueryRow(ctx, `
		SELECT
			COALESCE(SUM((status = 'Healthy')::int), 0),
			COUNT(*),
			COALESCE((SELECT status FROM monitored_contracts WHERE contract_id = $1), '')
		FROM (
			SELECT status FROM health_checks WHERE contract_id = $1 ORDER BY timestamp DESC LIMIT 100
		) recent`,
		contractID,
	)
	var healthyCount, totalCount int64
	var watchdogStatus string
	if err := row.Scan(&healthyCount, &totalCount, &watchdogStatus); err != nil {
		return in, fmt.Errorf("watchdog health inputs: %w", err)
	}
	in.HealthyChecks = healthyCount
	in.TotalChecks = totalCount
	in.WatchdogStatus = watchdogStatus

	// Invocation error rate over the trailing 24 hours.
	row = s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE status <> 'SUCCESS')
		FROM invocations
		WHERE contract_id = $1
		  AND ledger_closed_at >= NOW() - INTERVAL '24 hours'`,
		contractID,
	)
	var totalInv, failedInv int64
	if err := row.Scan(&totalInv, &failedInv); err != nil {
		return in, fmt.Errorf("invocation health inputs: %w", err)
	}
	in.TotalInvocations = totalInv
	in.FailedInvocations = failedInv

	// Performance trend comes from the same hourly activity the anomaly
	// detector uses.
	activity, err := s.RecentHourlyActivity(ctx, contractID, 24)
	if err != nil {
		return in, fmt.Errorf("activity health inputs: %w", err)
	}
	in.Activity = activity

	// Storage TTL headroom vs the contract's last known ledger.
	row = s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'live'),
			COUNT(*) FILTER (
				WHERE status = 'live'
				  AND live_until_ledger IS NOT NULL
				  AND live_until_ledger <= COALESCE((SELECT MAX(last_ledger) FROM sync_state WHERE contract_id = $1), 0) + $2
			)
		FROM storage_entries
		WHERE contract_id = $1`,
		contractID, storageHealthyHorizonLedgers,
	)
	var totalStorage, expiringStorage int64
	if err := row.Scan(&totalStorage, &expiringStorage); err != nil {
		return in, fmt.Errorf("storage health inputs: %w", err)
	}
	in.TotalStorage = totalStorage
	in.ExpiringStorage = expiringStorage

	return in, nil
}

func (s *postgresStore) UpsertContractHealthScore(ctx context.Context, h ContractHealthScore) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO contract_health_scores
			(contract_id, score, component_uptime, component_error_rate,
			 component_performance, component_storage_ttl, computed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (contract_id) DO UPDATE SET
			score                 = EXCLUDED.score,
			component_uptime      = EXCLUDED.component_uptime,
			component_error_rate  = EXCLUDED.component_error_rate,
			component_performance = EXCLUDED.component_performance,
			component_storage_ttl = EXCLUDED.component_storage_ttl,
			computed_at           = EXCLUDED.computed_at`,
		h.ContractID, h.Score, h.ComponentUptime, h.ComponentErrorRate,
		h.ComponentPerformance, h.ComponentStorageTTL, h.ComputedAt,
	)
	return err
}

func (s *postgresStore) GetContractHealthScore(ctx context.Context, contractID string) (ContractHealthScore, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT contract_id, score, component_uptime, component_error_rate,
		       component_performance, component_storage_ttl, computed_at
		FROM contract_health_scores
		WHERE contract_id = $1`, contractID)
	var h ContractHealthScore
	err := row.Scan(&h.ContractID, &h.Score, &h.ComponentUptime, &h.ComponentErrorRate,
		&h.ComponentPerformance, &h.ComponentStorageTTL, &h.ComputedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ContractHealthScore{}, ErrNotFound
	}
	if err != nil {
		return ContractHealthScore{}, fmt.Errorf("get contract health score: %w", err)
	}
	return h, nil
}