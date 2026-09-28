package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrRuleExists is returned when a rule with the same source and scope already
// exists. It maps to HTTP 409 in the handler.
var ErrRuleExists = errors.New("store: alert rule already exists")

// MetricSample is one per-minute bucket of metric values used to evaluate and
// preview alert rules. A metric that had no data in the bucket is absent.
type MetricSample struct {
	At     time.Time
	Values map[string]float64
}

// AlertRule is a user-defined rule in the Sorolens rule language
// (apps/api/rulelang). It corresponds to one row in the alert_rules table.
type AlertRule struct {
	ID         int64
	Name       string
	Source     string
	Severity   string // Info | Warning | Critical
	ContractID string // "" means every contract
	Network    string // "" means every network
	WindowSecs int64
	Enabled    bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// AlertRuleStore is the persistence surface for user-defined alert rules and
// the metric samples they are evaluated against.
type AlertRuleStore interface {
	// CreateAlertRule inserts a rule, returning it with its assigned ID.
	// Returns ErrRuleExists if the same source/scope pair is already stored.
	CreateAlertRule(ctx context.Context, r AlertRule) (AlertRule, error)
	// GetAlertRule returns one rule, or ErrNotFound.
	GetAlertRule(ctx context.Context, id int64) (AlertRule, error)
	// ListAlertRules returns every rule, newest first.
	ListAlertRules(ctx context.Context) ([]AlertRule, error)
	// SetAlertRuleEnabled enables or disables a rule, returning the updated row.
	SetAlertRuleEnabled(ctx context.Context, id int64, enabled bool) (AlertRule, error)
	// DeleteAlertRule removes a rule, returning ErrNotFound when it is absent.
	DeleteAlertRule(ctx context.Context, id int64) error
	// ContractMetricSamples returns per-minute metric buckets for a contract in
	// [from, to). It powers rule evaluation and the dashboard's live preview.
	ContractMetricSamples(ctx context.Context, contractID string, from, to time.Time) ([]MetricSample, error)
}

// ---- postgres implementation -----------------------------------------------

const alertRuleColumns = `id, name, source, severity, COALESCE(contract_id, ''), network,
	window_secs, enabled, created_at, updated_at`

func (s *postgresStore) CreateAlertRule(ctx context.Context, r AlertRule) (AlertRule, error) {
	now := time.Now().UTC()
	var contract *string
	if r.ContractID != "" {
		contract = &r.ContractID
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO alert_rules
		    (name, source, severity, contract_id, network, window_secs, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
		RETURNING `+alertRuleColumns,
		r.Name, r.Source, r.Severity, contract, r.Network, r.WindowSecs, r.Enabled, now,
	)
	out, err := scanAlertRule(row)
	if isUniqueViolation(err) {
		return AlertRule{}, ErrRuleExists
	}
	return out, err
}

func (s *postgresStore) GetAlertRule(ctx context.Context, id int64) (AlertRule, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+alertRuleColumns+` FROM alert_rules WHERE id = $1`, id)
	out, err := scanAlertRule(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return AlertRule{}, ErrNotFound
	}
	return out, err
}

func (s *postgresStore) ListAlertRules(ctx context.Context) ([]AlertRule, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+alertRuleColumns+` FROM alert_rules ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list alert rules: %w", err)
	}
	defer rows.Close()
	var out []AlertRule
	for rows.Next() {
		r, err := scanAlertRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *postgresStore) SetAlertRuleEnabled(ctx context.Context, id int64, enabled bool) (AlertRule, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE alert_rules SET enabled = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING `+alertRuleColumns, id, enabled)
	out, err := scanAlertRule(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return AlertRule{}, ErrNotFound
	}
	return out, err
}

func (s *postgresStore) DeleteAlertRule(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM alert_rules WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ContractMetricSamples aggregates events and invocations into one bucket per
// minute. Storage/health metrics are not available per minute here; those are
// evaluated by the indexer, which has the watchdog inputs already loaded.
func (s *postgresStore) ContractMetricSamples(ctx context.Context, contractID string, from, to time.Time) ([]MetricSample, error) {
	rows, err := s.pool.Query(ctx, `
		WITH buckets AS (
		    SELECT generate_series(
		        date_trunc('minute', $2::timestamptz),
		        date_trunc('minute', $3::timestamptz),
		        interval '1 minute') AS bucket
		),
		inv AS (
		    SELECT date_trunc('minute', ledger_closed_at) AS bucket,
		           count(*)::float8                                    AS invocations,
		           count(*) FILTER (WHERE status <> 'SUCCESS')::float8 AS failed_invocations,
		           COALESCE(sum(resource_fee_charged), 0)::float8      AS fee_total,
		           COALESCE(avg(resource_fee_charged), 0)::float8      AS fee_avg,
		           COALESCE(avg(cpu_insn), 0)::float8                  AS cpu_avg,
		           COALESCE(sum(cpu_insn), 0)::float8                  AS cpu_total,
		           COALESCE(avg(mem_byte), 0)::float8                  AS mem_avg,
		           COALESCE(sum(ledger_read_byte), 0)::float8          AS read_total,
		           COALESCE(sum(ledger_write_byte), 0)::float8         AS write_total
		    FROM invocations
		    WHERE contract_id = $1 AND ledger_closed_at >= $2 AND ledger_closed_at < $3
		    GROUP BY 1
		),
		ev AS (
		    SELECT date_trunc('minute', ledger_closed_at) AS bucket, count(*)::float8 AS events
		    FROM events
		    WHERE contract_id = $1 AND ledger_closed_at >= $2 AND ledger_closed_at < $3
		    GROUP BY 1
		)
		SELECT b.bucket,
		       COALESCE(inv.invocations, 0), COALESCE(inv.failed_invocations, 0),
		       COALESCE(inv.fee_total, 0), COALESCE(inv.fee_avg, 0),
		       COALESCE(inv.cpu_avg, 0), COALESCE(inv.cpu_total, 0), COALESCE(inv.mem_avg, 0),
		       COALESCE(inv.read_total, 0), COALESCE(inv.write_total, 0),
		       COALESCE(ev.events, 0)
		FROM buckets b
		LEFT JOIN inv ON inv.bucket = b.bucket
		LEFT JOIN ev  ON ev.bucket  = b.bucket
		ORDER BY b.bucket`,
		contractID, from.UTC(), to.UTC())
	if err != nil {
		return nil, fmt.Errorf("contract metric samples: %w", err)
	}
	defer rows.Close()

	var out []MetricSample
	for rows.Next() {
		var (
			at                                            time.Time
			invocations, failed, feeTotal, feeAvg         float64
			cpuAvg, cpuTotal, memAvg, readTotal, writeTot float64
			events                                        float64
		)
		if err := rows.Scan(&at, &invocations, &failed, &feeTotal, &feeAvg,
			&cpuAvg, &cpuTotal, &memAvg, &readTotal, &writeTot, &events); err != nil {
			return nil, err
		}
		out = append(out, MetricSample{At: at.UTC(), Values: invocationMetricValues(
			invocations, failed, feeTotal, feeAvg, cpuAvg, cpuTotal, memAvg, readTotal, writeTot, events)})
	}
	return out, rows.Err()
}

// invocationMetricValues maps one aggregated bucket onto the rule metric
// names. Missing signals are omitted rather than reported as zero, so a
// rule's "no data" path stays honest.
func invocationMetricValues(invocations, failed, feeTotal, feeAvg, cpuAvg, cpuTotal, memAvg, readTotal, writeTotal, events float64) map[string]float64 {
	const stroopsPerXLM = 1e7
	v := map[string]float64{
		"invocations":        invocations,
		"failed_invocations": failed,
		"events":             events,
		"total_fee":          feeTotal / stroopsPerXLM,
	}
	if invocations > 0 {
		v["error_rate"] = failed / invocations
		v["fee_per_invocation"] = feeAvg / stroopsPerXLM
		v["fee_per_invocation_stroops"] = feeAvg
		v["cpu_insn_per_invocation"] = cpuAvg
		v["cpu_insn_total"] = cpuTotal
		v["mem_byte_per_invocation"] = memAvg
		v["ledger_read_bytes"] = readTotal
		v["ledger_write_bytes"] = writeTotal
	} else if cpuTotal > 0 {
		v["cpu_insn_total"] = cpuTotal
	}
	return v
}

func scanAlertRule(row pgx.Row) (AlertRule, error) {
	var r AlertRule
	err := row.Scan(&r.ID, &r.Name, &r.Source, &r.Severity, &r.ContractID, &r.Network,
		&r.WindowSecs, &r.Enabled, &r.CreatedAt, &r.UpdatedAt)
	return r, err
}

// isUniqueViolation reports whether err is a Postgres unique-constraint error.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// ---- MockStore implementation ----------------------------------------------

func (m *MockStore) CreateAlertRule(_ context.Context, r AlertRule) (AlertRule, error) {
	if m.CreateAlertRuleErr != nil {
		return AlertRule{}, m.CreateAlertRuleErr
	}
	for _, existing := range m.alertRules {
		if existing.Source == r.Source && existing.ContractID == r.ContractID && existing.Network == r.Network {
			return AlertRule{}, ErrRuleExists
		}
	}
	r.ID = m.nextRuleID()
	now := time.Now().UTC()
	if r.CreatedAt.IsZero() {
		r.CreatedAt = now
	}
	r.UpdatedAt = now
	m.alertRules = append(m.alertRules, r)
	return r, nil
}

func (m *MockStore) nextRuleID() int64 {
	var maxID int64
	for _, r := range m.alertRules {
		if r.ID > maxID {
			maxID = r.ID
		}
	}
	return maxID + 1
}

func (m *MockStore) GetAlertRule(_ context.Context, id int64) (AlertRule, error) {
	if m.GetAlertRuleErr != nil {
		return AlertRule{}, m.GetAlertRuleErr
	}
	for _, r := range m.alertRules {
		if r.ID == id {
			return r, nil
		}
	}
	return AlertRule{}, ErrNotFound
}

func (m *MockStore) ListAlertRules(_ context.Context) ([]AlertRule, error) {
	if m.ListAlertRulesErr != nil {
		return nil, m.ListAlertRulesErr
	}
	out := make([]AlertRule, len(m.alertRules))
	copy(out, m.alertRules)
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func (m *MockStore) SetAlertRuleEnabled(_ context.Context, id int64, enabled bool) (AlertRule, error) {
	for i := range m.alertRules {
		if m.alertRules[i].ID == id {
			m.alertRules[i].Enabled = enabled
			m.alertRules[i].UpdatedAt = time.Now().UTC()
			return m.alertRules[i], nil
		}
	}
	return AlertRule{}, ErrNotFound
}

func (m *MockStore) DeleteAlertRule(_ context.Context, id int64) error {
	for i := range m.alertRules {
		if m.alertRules[i].ID == id {
			m.alertRules = append(m.alertRules[:i], m.alertRules[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

// ContractMetricSamples mirrors the postgres per-minute aggregation using the
// in-memory events and invocations.
func (m *MockStore) ContractMetricSamples(_ context.Context, contractID string, from, to time.Time) ([]MetricSample, error) {
	if m.ContractMetricSamplesErr != nil {
		return nil, m.ContractMetricSamplesErr
	}
	from = from.UTC().Truncate(time.Minute)
	to = to.UTC().Truncate(time.Minute)
	if !to.After(from) {
		return nil, nil
	}

	type bucket struct {
		invocations, failed, feeTotal, feeAvgNum, cpuTotal, cpuAvgNum float64
		memAvgNum, readTotal, writeTotal, events                      float64
	}
	buckets := map[time.Time]*bucket{}
	bucketOf := func(t time.Time) time.Time { return t.UTC().Truncate(time.Minute) }

	for _, inv := range m.invocations {
		if inv.ContractID != contractID || inv.LedgerClosedAt.Before(from) || !inv.LedgerClosedAt.Before(to) {
			continue
		}
		b := buckets[bucketOf(inv.LedgerClosedAt)]
		if b == nil {
			b = &bucket{}
			buckets[bucketOf(inv.LedgerClosedAt)] = b
		}
		b.invocations++
		if inv.Status != "SUCCESS" {
			b.failed++
		}
		b.feeTotal += float64(inv.ResourceFeeCharged)
		b.feeAvgNum += float64(inv.ResourceFeeCharged)
		b.cpuTotal += float64(inv.CPUInsn)
		b.cpuAvgNum += float64(inv.CPUInsn)
		b.memAvgNum += float64(inv.MemByte)
		b.readTotal += float64(inv.LedgerReadByte)
		b.writeTotal += float64(inv.LedgerWriteByte)
	}
	for _, e := range m.events {
		if e.ContractID != contractID || e.LedgerClosedAt.Before(from) || !e.LedgerClosedAt.Before(to) {
			continue
		}
		b := buckets[bucketOf(e.LedgerClosedAt)]
		if b == nil {
			b = &bucket{}
			buckets[bucketOf(e.LedgerClosedAt)] = b
		}
		b.events++
	}

	var out []MetricSample
	for at := from; at.Before(to); at = at.Add(time.Minute) {
		b := buckets[at]
		if b == nil {
			out = append(out, MetricSample{At: at, Values: map[string]float64{
				"invocations": 0, "failed_invocations": 0, "events": 0, "total_fee": 0,
			}})
			continue
		}
		feeAvg, cpuAvg, memAvg := 0.0, 0.0, 0.0
		if b.invocations > 0 {
			feeAvg = b.feeAvgNum / b.invocations
			cpuAvg = b.cpuAvgNum / b.invocations
			memAvg = b.memAvgNum / b.invocations
		}
		out = append(out, MetricSample{At: at, Values: invocationMetricValues(
			b.invocations, b.failed, b.feeTotal, feeAvg, cpuAvg, b.cpuTotal, memAvg, b.readTotal, b.writeTotal, b.events)})
	}
	return out, nil
}
