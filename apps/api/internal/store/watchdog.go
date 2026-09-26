package store

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ---- models ----------------------------------------------------------------

// MonitoredContract mirrors a row in monitored_contracts. It's the current
// health snapshot for one contract registered with the on-chain watchdog.
type MonitoredContract struct {
	ContractID    string
	Network       string
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

// UptimeResult holds the computed uptime percentage for a given time window.
type UptimeResult struct {
	ContractID string  `json:"contract_id"`
	Window     string  `json:"window"`
	Uptime     float64 `json:"uptime_pct"`
}

// ---- interface -------------------------------------------------------------

// ErrInvalidCursor is returned when a pagination cursor cannot be decoded
// into the (timestamp, id) keyset pair it must carry.
var ErrInvalidCursor = errors.New("store: invalid pagination cursor")

// encodeAlertsCursor builds the opaque cursor for the alerts feed: a
// base64-encoded "<RFC3339Nano timestamp>|<contract_id>" pair pointing at
// the last row of the returned page (issue #150).
func encodeAlertsCursor(ts time.Time, contractID string) string {
	raw := ts.UTC().Format(time.RFC3339Nano) + "|" + contractID
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

// decodeAlertsCursor parses a cursor produced by encodeAlertsCursor back
// into its (timestamp, contractID) pair.
func decodeAlertsCursor(cursor string) (time.Time, string, error) {
	if cursor == "" {
		return time.Time{}, "", ErrInvalidCursor
	}
	raw, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", ErrInvalidCursor
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return time.Time{}, "", ErrInvalidCursor
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", ErrInvalidCursor
	}
	return ts, parts[1], nil
}

// WatchdogStore is the read/write surface for watchdog data. Kept as its own
// interface so the API can be wired without pulling watchdog handlers into
// tests that don't need them.
type WatchdogStore interface {
	UpsertMonitoredContract(ctx context.Context, m MonitoredContract) error
	DeleteMonitoredContract(ctx context.Context, contractID string) error
	InsertHealthCheck(ctx context.Context, h HealthCheck) error
	InsertContractAlert(ctx context.Context, a ContractAlert) error

	ListMonitoredContracts(ctx context.Context, cursor string, limit int, network string) ([]MonitoredContract, string, error)
	GetMonitoredContract(ctx context.Context, contractID string) (MonitoredContract, error)
	ListHealthChecks(ctx context.Context, contractID string, limit int) ([]HealthCheck, error)
	ListAlerts(ctx context.Context, contractID, severity, network, cursor string, limit int) ([]ContractAlert, string, error)
	GetWatchdogStats(ctx context.Context, network string) (WatchdogStats, error)

	// GetContractUptime computes the uptime percentage for the given contract
	// over the requested time window ("24h", "7d", or "30d"). Uptime is
	// defined as the fraction of health-check timestamps within the window
	// where the status was "Healthy".
	GetContractUptime(ctx context.Context, contractID string, window string) (UptimeResult, error)
}

// ---- postgres implementation ----------------------------------------------

func (s *postgresStore) UpsertMonitoredContract(ctx context.Context, m MonitoredContract) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO monitored_contracts
			(contract_id, network, name, owner, status, last_check, check_interval, registered_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (contract_id) DO UPDATE SET
			network        = EXCLUDED.network,
			name           = EXCLUDED.name,
			owner          = EXCLUDED.owner,
			status         = EXCLUDED.status,
			last_check     = COALESCE(EXCLUDED.last_check, monitored_contracts.last_check),
			check_interval = EXCLUDED.check_interval,
			updated_at     = EXCLUDED.updated_at`,
		m.ContractID, networkOrDefault(m.Network), m.Name, m.Owner, m.Status, m.LastCheck,
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

func (s *postgresStore) ListMonitoredContracts(ctx context.Context, cursor string, limit int, network string) ([]MonitoredContract, string, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT contract_id, network, name, owner, status, last_check, check_interval, registered_at, updated_at
		FROM monitored_contracts
		WHERE ($1 = '' OR contract_id > $1)
		  AND ($2 = '' OR network = $2)
		ORDER BY contract_id ASC
		LIMIT $3`, cursor, network, limit+1)
	if err != nil {
		return nil, "", fmt.Errorf("list monitored contracts: %w", err)
	}
	defer rows.Close()

	var out []MonitoredContract
	for rows.Next() {
		var m MonitoredContract
		if err := rows.Scan(&m.ContractID, &m.Network, &m.Name, &m.Owner, &m.Status,
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
		SELECT contract_id, network, name, owner, status, last_check, check_interval, registered_at, updated_at
		FROM monitored_contracts WHERE contract_id = $1`, contractID)
	var m MonitoredContract
	err := row.Scan(&m.ContractID, &m.Network, &m.Name, &m.Owner, &m.Status,
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

// ListAlerts returns one page of the alerts feed, newest first. The
// optional cursor ("" for the first page) is a (timestamp, contract_id)
// keyset pair; rows strictly older than the pair are returned, so paging
// stays stable when new alerts arrive between requests (issue #150).
//
// The returned cursor points at the last row of this page and is empty when
// the feed is exhausted. An undecodable cursor yields ErrInvalidCursor.
func (s *postgresStore) ListAlerts(ctx context.Context, contractID, severity, network, cursor string, limit int) ([]ContractAlert, string, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var cursorTS time.Time
	var cursorID string
	if cursor != "" {
		ts, id, err := decodeAlertsCursor(cursor)
		if err != nil {
			return nil, "", err
		}
		cursorTS, cursorID = ts, id
	}
	rows, err := s.pool.Query(ctx, `
		SELECT a.contract_id, a.severity, a.message, a.ledger, a.tx_hash, a.timestamp
		FROM contract_alerts a
		JOIN monitored_contracts m ON m.contract_id = a.contract_id
		WHERE ($1 = '' OR a.contract_id = $1)
		  AND ($2 = '' OR a.severity = $2)
		  AND ($3 = '' OR m.network = $3)
		  AND ($4 = '' OR (a.timestamp, a.contract_id) < ($4::timestamptz, $5))
		ORDER BY a.timestamp DESC, a.contract_id DESC
		LIMIT $6`, contractID, severity, network,
		cursorTS.Format(time.RFC3339Nano), cursorID, limit+1)
	if err != nil {
		return nil, "", fmt.Errorf("list alerts: %w", err)
	}
	defer rows.Close()

	var out []ContractAlert
	for rows.Next() {
		var a ContractAlert
		if err := rows.Scan(&a.ContractID, &a.Severity, &a.Message, &a.Ledger, &a.TxHash, &a.Timestamp); err != nil {
			return nil, "", err
		}
		out = append(out, a)
	}
	if rows.Err() != nil {
		return nil, "", rows.Err()
	}
	var next string
	if len(out) > limit {
		next = encodeAlertsCursor(out[limit-1].Timestamp, out[limit-1].ContractID)
		out = out[:limit]
	}
	return out, next, nil
}

func (s *postgresStore) GetWatchdogStats(ctx context.Context, network string) (WatchdogStats, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM monitored_contracts WHERE ($1 = '' OR network = $1))                              AS total_monitored,
			(SELECT COUNT(*) FROM monitored_contracts WHERE status = 'Healthy'      AND ($1 = '' OR network = $1)) AS healthy,
			(SELECT COUNT(*) FROM monitored_contracts WHERE status = 'Degraded'     AND ($1 = '' OR network = $1)) AS degraded,
			(SELECT COUNT(*) FROM monitored_contracts WHERE status = 'Unresponsive' AND ($1 = '' OR network = $1)) AS unresponsive,
			(SELECT COUNT(*) FROM contract_alerts a JOIN monitored_contracts m ON m.contract_id = a.contract_id
			  WHERE ($1 = '' OR m.network = $1))                                                                   AS total_alerts,
			(SELECT COUNT(*) FROM contract_alerts a JOIN monitored_contracts m ON m.contract_id = a.contract_id
			  WHERE a.severity = 'Critical' AND ($1 = '' OR m.network = $1))                                      AS critical_alerts`,
		network)
	var s2 WatchdogStats
	err := row.Scan(&s2.TotalMonitored, &s2.Healthy, &s2.Degraded, &s2.Unresponsive,
		&s2.TotalAlerts, &s2.CriticalAlerts)
	return s2, err
}

// windowDuration maps a string window token to a Postgres interval string.
func windowDuration(window string) (string, bool) {
	switch window {
	case "24h":
		return "24 hours", true
	case "7d":
		return "7 days", true
	case "30d":
		return "30 days", true
	default:
		return "", false
	}
}

func (s *postgresStore) GetContractUptime(ctx context.Context, contractID string, window string) (UptimeResult, error) {
	interval, ok := windowDuration(window)
	if !ok {
		return UptimeResult{}, fmt.Errorf("invalid window %q: must be 24h, 7d, or 30d", window)
	}

	row := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'Healthy') AS healthy_count,
			COUNT(*)                                    AS total_count
		FROM health_checks
		WHERE contract_id = $1
		  AND timestamp >= NOW() - $2::interval`,
		contractID, interval)

	var healthyCount, totalCount int64
	if err := row.Scan(&healthyCount, &totalCount); err != nil {
		return UptimeResult{}, fmt.Errorf("get contract uptime: %w", err)
	}

	var uptimePct float64
	if totalCount > 0 {
		uptimePct = float64(healthyCount) / float64(totalCount) * 100.0
	}

	return UptimeResult{
		ContractID: contractID,
		Window:     window,
		Uptime:     uptimePct,
	}, nil
}
