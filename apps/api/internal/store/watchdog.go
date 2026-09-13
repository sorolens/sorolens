package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ---- models ----------------------------------------------------------------

// MonitoredContract mirrors a row in monitored_contracts. It's the current
// health snapshot for one contract registered with the on-chain watchdog.
type MonitoredContract struct {
	ContractID    string
	Name          string
	Owner         string
	Status        string // Healthy | Degraded | Unresponsive | <custom>
	LastCheck     *time.Time
	CheckInterval int64
	RegisteredAt  time.Time
	UpdatedAt     time.Time
}

// HealthCheck is a single health status push from a monitored contract.
type HealthCheck struct {
	ContractID string
	Status     string
	Metadata   string
	Ledger     int64
	TxHash     string
	Timestamp  time.Time
}

// ContractAlert is a single alert emitted for a monitored contract.
type ContractAlert struct {
	ContractID string
	Severity   string // Info | Warning | Critical
	Message    string
	Ledger     int64
	TxHash     string
	Timestamp  time.Time
}

// WatchdogStats is the summary shown on the watchdog dashboard.
type WatchdogStats struct {
	TotalMonitored int64
	Healthy        int64
	Degraded       int64
	Unresponsive   int64
	TotalAlerts    int64
	CriticalAlerts int64
}

// ---- interface -------------------------------------------------------------

// WatchdogStore is the read/write surface for watchdog data. Kept as its own
// interface so the API can be wired without pulling watchdog handlers into
// tests that don't need them.
type WatchdogStore interface {
	UpsertMonitoredContract(ctx context.Context, m MonitoredContract) error
	DeleteMonitoredContract(ctx context.Context, contractID string) error
	InsertHealthCheck(ctx context.Context, h HealthCheck) error
	InsertContractAlert(ctx context.Context, a ContractAlert) error

	ListMonitoredContracts(ctx context.Context, cursor string, limit int) ([]MonitoredContract, string, error)
	GetMonitoredContract(ctx context.Context, contractID string) (MonitoredContract, error)
	ListHealthChecks(ctx context.Context, contractID string, limit int) ([]HealthCheck, error)
	ListAlerts(ctx context.Context, contractID, severity string, limit int) ([]ContractAlert, error)
	GetWatchdogStats(ctx context.Context) (WatchdogStats, error)
}

// ---- postgres implementation ----------------------------------------------

func (s *postgresStore) UpsertMonitoredContract(ctx context.Context, m MonitoredContract) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO monitored_contracts
			(contract_id, name, owner, status, last_check, check_interval, registered_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (contract_id) DO UPDATE SET
			name           = EXCLUDED.name,
			owner          = EXCLUDED.owner,
			status         = EXCLUDED.status,
			last_check     = COALESCE(EXCLUDED.last_check, monitored_contracts.last_check),
			check_interval = EXCLUDED.check_interval,
			updated_at     = EXCLUDED.updated_at`,
		m.ContractID, m.Name, m.Owner, m.Status, m.LastCheck,
		m.CheckInterval, m.RegisteredAt, time.Now().UTC(),
	)
	return err
}

func (s *postgresStore) DeleteMonitoredContract(ctx context.Context, contractID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM monitored_contracts WHERE contract_id = $1`, contractID)
	return err
}

func (s *postgresStore) InsertHealthCheck(ctx context.Context, h HealthCheck) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO health_checks (contract_id, status, metadata, ledger, tx_hash, timestamp)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (tx_hash, contract_id) DO NOTHING`,
		h.ContractID, h.Status, h.Metadata, h.Ledger, h.TxHash, h.Timestamp,
	)
	return err
}

func (s *postgresStore) InsertContractAlert(ctx context.Context, a ContractAlert) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO contract_alerts (contract_id, severity, message, ledger, tx_hash, timestamp)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (tx_hash, contract_id) DO NOTHING`,
		a.ContractID, a.Severity, a.Message, a.Ledger, a.TxHash, a.Timestamp,
	)
	return err
}

func (s *postgresStore) ListMonitoredContracts(ctx context.Context, cursor string, limit int) ([]MonitoredContract, string, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT contract_id, name, owner, status, last_check, check_interval, registered_at, updated_at
		FROM monitored_contracts
		WHERE ($1 = '' OR contract_id > $1)
		ORDER BY contract_id ASC
		LIMIT $2`, cursor, limit+1)
	if err != nil {
		return nil, "", fmt.Errorf("list monitored contracts: %w", err)
	}
	defer rows.Close()

	var out []MonitoredContract
	for rows.Next() {
		var m MonitoredContract
		if err := rows.Scan(&m.ContractID, &m.Name, &m.Owner, &m.Status,
			&m.LastCheck, &m.CheckInterval, &m.RegisteredAt, &m.UpdatedAt); err != nil {
			return nil, "", err
		}
		out = append(out, m)
	}
	if rows.Err() != nil {
		return nil, "", rows.Err()
	}
	var next string
	if len(out) > limit {
		next = out[limit-1].ContractID
		out = out[:limit]
	}
	return out, next, nil
}

func (s *postgresStore) GetMonitoredContract(ctx context.Context, contractID string) (MonitoredContract, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT contract_id, name, owner, status, last_check, check_interval, registered_at, updated_at
		FROM monitored_contracts WHERE contract_id = $1`, contractID)
	var m MonitoredContract
	err := row.Scan(&m.ContractID, &m.Name, &m.Owner, &m.Status,
		&m.LastCheck, &m.CheckInterval, &m.RegisteredAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return MonitoredContract{}, ErrNotFound
	}
	return m, err
}

func (s *postgresStore) ListHealthChecks(ctx context.Context, contractID string, limit int) ([]HealthCheck, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT contract_id, status, metadata, ledger, tx_hash, timestamp
		FROM health_checks
		WHERE contract_id = $1
		ORDER BY timestamp DESC
		LIMIT $2`, contractID, limit)
	if err != nil {
		return nil, fmt.Errorf("list health checks: %w", err)
	}
	defer rows.Close()

	var out []HealthCheck
	for rows.Next() {
		var h HealthCheck
		var meta *string
		if err := rows.Scan(&h.ContractID, &h.Status, &meta, &h.Ledger, &h.TxHash, &h.Timestamp); err != nil {
			return nil, err
		}
		if meta != nil {
			h.Metadata = *meta
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *postgresStore) ListAlerts(ctx context.Context, contractID, severity string, limit int) ([]ContractAlert, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT contract_id, severity, message, ledger, tx_hash, timestamp
		FROM contract_alerts
		WHERE ($1 = '' OR contract_id = $1)
		  AND ($2 = '' OR severity = $2)
		ORDER BY timestamp DESC
		LIMIT $3`, contractID, severity, limit)
	if err != nil {
		return nil, fmt.Errorf("list alerts: %w", err)
	}
	defer rows.Close()

	var out []ContractAlert
	for rows.Next() {
		var a ContractAlert
		if err := rows.Scan(&a.ContractID, &a.Severity, &a.Message, &a.Ledger, &a.TxHash, &a.Timestamp); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *postgresStore) GetWatchdogStats(ctx context.Context) (WatchdogStats, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM monitored_contracts)                                AS total_monitored,
			(SELECT COUNT(*) FROM monitored_contracts WHERE status = 'Healthy')       AS healthy,
			(SELECT COUNT(*) FROM monitored_contracts WHERE status = 'Degraded')      AS degraded,
			(SELECT COUNT(*) FROM monitored_contracts WHERE status = 'Unresponsive')  AS unresponsive,
			(SELECT COUNT(*) FROM contract_alerts)                                    AS total_alerts,
			(SELECT COUNT(*) FROM contract_alerts WHERE severity = 'Critical')        AS critical_alerts`)
	var s2 WatchdogStats
	err := row.Scan(&s2.TotalMonitored, &s2.Healthy, &s2.Degraded, &s2.Unresponsive,
		&s2.TotalAlerts, &s2.CriticalAlerts)
	return s2, err
}
