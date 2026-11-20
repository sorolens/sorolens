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

// Alert lifecycle state constants (issue #789).
const (
	AlertStateOpen         = "open"
	AlertStateAcknowledged = "acknowledged"
	AlertStateSnoozed      = "snoozed"
	AlertStateResolved     = "resolved"
)

var (
	ErrInvalidTransition = errors.New("store: invalid alert state transition")
	ErrAlertNotFound      = errors.New("store: alert not found")
)

// ContractAlert is a single alert emitted for a monitored contract with lifecycle state.
type ContractAlert struct {
	ID                  int64
	ContractID          string
	Severity            string // Info | Warning | Critical
	Message             string
	Ledger              int64
	TxHash              string
	Timestamp           time.Time
	State               string // open | acknowledged | snoozed | resolved
	AcknowledgedAt      *time.Time
	AcknowledgedBy      string
	SnoozedUntil        *time.Time
	SnoozedAt           *time.Time
	SnoozedBy           string
	ResolvedAt          *time.Time
	ResolvedBy          string
	MaintenanceWindowID *int64
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
	ListAlertsWithState(ctx context.Context, contractID, severity, network, state, cursor string, limit int) ([]ContractAlert, string, error)
	AcknowledgeAlert(ctx context.Context, id int64, user string) (ContractAlert, error)
	SnoozeAlert(ctx context.Context, id int64, duration time.Duration, user string) (ContractAlert, error)
	ResolveAlert(ctx context.Context, id int64, user string) (ContractAlert, error)
	GetAlertCountsByState(ctx context.Context, contractID, network string) (map[string]int64, error)
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
	st := a.State
	if st == "" {
		st = AlertStateOpen
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO contract_alerts (
			contract_id, severity, message, ledger, tx_hash, timestamp,
			state, maintenance_window_id
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (tx_hash, contract_id) DO NOTHING`,
		a.ContractID, a.Severity, a.Message, a.Ledger, a.TxHash, a.Timestamp,
		st, a.MaintenanceWindowID,
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

type scannableRow interface {
	Scan(dest ...any) error
}

func scanContractAlertRow(row scannableRow, a *ContractAlert) error {
	var ackBy, snzBy, resBy *string
	err := row.Scan(
		&a.ID, &a.ContractID, &a.Severity, &a.Message, &a.Ledger, &a.TxHash, &a.Timestamp,
		&a.State, &a.AcknowledgedAt, &ackBy, &a.SnoozedUntil, &a.SnoozedAt, &snzBy,
		&a.ResolvedAt, &resBy, &a.MaintenanceWindowID,
	)
	if err != nil {
		return err
	}
	if ackBy != nil {
		a.AcknowledgedBy = *ackBy
	}
	if snzBy != nil {
		a.SnoozedBy = *snzBy
	}
	if resBy != nil {
		a.ResolvedBy = *resBy
	}
	// Snooze expiration: if snoozed_until is in the past, effectively open
	if a.State == AlertStateSnoozed && a.SnoozedUntil != nil && a.SnoozedUntil.Before(time.Now().UTC()) {
		a.State = AlertStateOpen
	}
	return nil
}

// ListAlerts delegates to ListAlertsWithState with an empty state filter.
func (s *postgresStore) ListAlerts(ctx context.Context, contractID, severity, network, cursor string, limit int) ([]ContractAlert, string, error) {
	return s.ListAlertsWithState(ctx, contractID, severity, network, "", cursor, limit)
}

// ListAlertsWithState returns one page of the alerts feed, newest first, with optional state filtering.
func (s *postgresStore) ListAlertsWithState(ctx context.Context, contractID, severity, network, state, cursor string, limit int) ([]ContractAlert, string, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var cursorTS time.Time
	var cursorID string
	hasCursor := cursor != ""
	if hasCursor {
		ts, id, err := decodeAlertsCursor(cursor)
		if err != nil {
			return nil, "", err
		}
		cursorTS, cursorID = ts, id
	}
	rows, err := s.pool.Query(ctx, `
		SELECT a.id, a.contract_id, a.severity, a.message, a.ledger, a.tx_hash, a.timestamp,
		       a.state, a.acknowledged_at, a.acknowledged_by, a.snoozed_until, a.snoozed_at, a.snoozed_by,
		       a.resolved_at, a.resolved_by, a.maintenance_window_id
		FROM contract_alerts a
		JOIN monitored_contracts m ON m.contract_id = a.contract_id
		WHERE ($1 = '' OR a.contract_id = $1)
		  AND ($2 = '' OR a.severity = $2)
		  AND ($3 = '' OR m.network = $3)
		  AND ($4 = '' OR a.state = $4)
		  AND (NOT $5 OR (a.timestamp, a.contract_id) < ($6::timestamptz, $7))
		ORDER BY a.timestamp DESC, a.contract_id DESC
		LIMIT $8`, contractID, severity, network, state,
		hasCursor, cursorTS, cursorID, limit+1)
	if err != nil {
		return nil, "", fmt.Errorf("list alerts: %w", err)
	}
	defer rows.Close()

	var out []ContractAlert
	for rows.Next() {
		var a ContractAlert
		if err := scanContractAlertRow(rows, &a); err != nil {
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

func (s *postgresStore) AcknowledgeAlert(ctx context.Context, id int64, user string) (ContractAlert, error) {
	var a ContractAlert
	row := s.pool.QueryRow(ctx, `
		SELECT a.id, a.contract_id, a.severity, a.message, a.ledger, a.tx_hash, a.timestamp,
		       a.state, a.acknowledged_at, a.acknowledged_by, a.snoozed_until, a.snoozed_at, a.snoozed_by,
		       a.resolved_at, a.resolved_by, a.maintenance_window_id
		FROM contract_alerts a
		WHERE a.id = $1`, id)
	err := scanContractAlertRow(row, &a)
	if errors.Is(err, pgx.ErrNoRows) {
		return ContractAlert{}, ErrAlertNotFound
	} else if err != nil {
		return ContractAlert{}, err
	}

	if a.State == AlertStateResolved {
		return ContractAlert{}, fmt.Errorf("%w: cannot acknowledge an already resolved alert", ErrInvalidTransition)
	}

	now := time.Now().UTC()
	updateRow := s.pool.QueryRow(ctx, `
		UPDATE contract_alerts
		SET state = $2, acknowledged_at = $3, acknowledged_by = $4
		WHERE id = $1
		RETURNING id, contract_id, severity, message, ledger, tx_hash, timestamp,
		          state, acknowledged_at, acknowledged_by, snoozed_until, snoozed_at, snoozed_by,
		          resolved_at, resolved_by, maintenance_window_id`,
		id, AlertStateAcknowledged, now, user)
	err = scanContractAlertRow(updateRow, &a)
	return a, err
}

func (s *postgresStore) SnoozeAlert(ctx context.Context, id int64, duration time.Duration, user string) (ContractAlert, error) {
	if duration <= 0 {
		return ContractAlert{}, fmt.Errorf("%w: snooze duration must be positive", ErrInvalidTransition)
	}
	var a ContractAlert
	row := s.pool.QueryRow(ctx, `
		SELECT a.id, a.contract_id, a.severity, a.message, a.ledger, a.tx_hash, a.timestamp,
		       a.state, a.acknowledged_at, a.acknowledged_by, a.snoozed_until, a.snoozed_at, a.snoozed_by,
		       a.resolved_at, a.resolved_by, a.maintenance_window_id
		FROM contract_alerts a
		WHERE a.id = $1`, id)
	err := scanContractAlertRow(row, &a)
	if errors.Is(err, pgx.ErrNoRows) {
		return ContractAlert{}, ErrAlertNotFound
	} else if err != nil {
		return ContractAlert{}, err
	}

	if a.State == AlertStateResolved {
		return ContractAlert{}, fmt.Errorf("%w: cannot snooze an already resolved alert", ErrInvalidTransition)
	}

	now := time.Now().UTC()
	until := now.Add(duration)
	updateRow := s.pool.QueryRow(ctx, `
		UPDATE contract_alerts
		SET state = $2, snoozed_at = $3, snoozed_by = $4, snoozed_until = $5
		WHERE id = $1
		RETURNING id, contract_id, severity, message, ledger, tx_hash, timestamp,
		          state, acknowledged_at, acknowledged_by, snoozed_until, snoozed_at, snoozed_by,
		          resolved_at, resolved_by, maintenance_window_id`,
		id, AlertStateSnoozed, now, user, until)
	err = scanContractAlertRow(updateRow, &a)
	return a, err
}

func (s *postgresStore) ResolveAlert(ctx context.Context, id int64, user string) (ContractAlert, error) {
	var a ContractAlert
	row := s.pool.QueryRow(ctx, `
		SELECT a.id, a.contract_id, a.severity, a.message, a.ledger, a.tx_hash, a.timestamp,
		       a.state, a.acknowledged_at, a.acknowledged_by, a.snoozed_until, a.snoozed_at, a.snoozed_by,
		       a.resolved_at, a.resolved_by, a.maintenance_window_id
		FROM contract_alerts a
		WHERE a.id = $1`, id)
	err := scanContractAlertRow(row, &a)
	if errors.Is(err, pgx.ErrNoRows) {
		return ContractAlert{}, ErrAlertNotFound
	} else if err != nil {
		return ContractAlert{}, err
	}

	if a.State == AlertStateResolved {
		return ContractAlert{}, fmt.Errorf("%w: alert is already resolved", ErrInvalidTransition)
	}

	now := time.Now().UTC()
	updateRow := s.pool.QueryRow(ctx, `
		UPDATE contract_alerts
		SET state = $2, resolved_at = $3, resolved_by = $4
		WHERE id = $1
		RETURNING id, contract_id, severity, message, ledger, tx_hash, timestamp,
		          state, acknowledged_at, acknowledged_by, snoozed_until, snoozed_at, snoozed_by,
		          resolved_at, resolved_by, maintenance_window_id`,
		id, AlertStateResolved, now, user)
	err = scanContractAlertRow(updateRow, &a)
	return a, err
}

func (s *postgresStore) GetAlertCountsByState(ctx context.Context, contractID, network string) (map[string]int64, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			CASE
				WHEN a.state = 'snoozed' AND a.snoozed_until IS NOT NULL AND a.snoozed_until <= NOW() THEN 'open'
				ELSE a.state
			END AS effective_state,
			COUNT(*)
		FROM contract_alerts a
		JOIN monitored_contracts m ON m.contract_id = a.contract_id
		WHERE ($1 = '' OR a.contract_id = $1)
		  AND ($2 = '' OR m.network = $2)
		GROUP BY effective_state`, contractID, network)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int64{
		AlertStateOpen:         0,
		AlertStateAcknowledged: 0,
		AlertStateSnoozed:      0,
		AlertStateResolved:     0,
	}
	for rows.Next() {
		var st string
		var cnt int64
		if err := rows.Scan(&st, &cnt); err != nil {
			return nil, err
		}
		counts[st] = cnt
	}
	return counts, rows.Err()
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
