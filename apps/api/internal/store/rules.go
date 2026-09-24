package store

import (
	"context"
	"time"

	"github.com/sorolens/sorolens/packages/rules"
)

// AlertRule is one user-defined alert-rule row. It stores the raw DSL
// expression plus a few denormalized fields (contract scope, severity,
// enabled flag) the indexer and dashboard use for fast filtering.
type AlertRule struct {
	ID         string
	Name       string
	Expression string
	// ContractID is "" when the rule applies to every tracked contract; the
	// indexer still evaluates it once per contract.
	ContractID string
	Network    string
	Severity   string // Info | Warning | Critical
	Enabled    bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// RuleStore is the read/write surface for user-defined alert rules.
type RuleStore interface {
	// CreateRule inserts a new rule.
	CreateRule(ctx context.Context, r AlertRule) error
	// UpdateRule overwrites mutable rule fields (name, expression, scope,
	// severity, enabled) keyed by ID.
	UpdateRule(ctx context.Context, r AlertRule) error
	// DeleteRule removes a rule by ID, or returns ErrNotFound.
	DeleteRule(ctx context.Context, id string) error
	// GetRule returns one rule by ID, or ErrNotFound.
	GetRule(ctx context.Context, id string) (AlertRule, error)
	// ListRules is a cursor-paginated rule list. An empty enabled filter
	// returns every rule; a non-empty contractID filters narrowly.
	ListRules(ctx context.Context, cursor string, limit int, enabled *bool, contractID string) ([]AlertRule, string, error)
	// RuleWindowStats fetches per-invocation samples plus the event count for
	// one contract over the trailing window, feeding the DSL evaluator.
	RuleWindowStats(ctx context.Context, contractID string, window time.Duration) (rules.WindowStats, error)
}

// ruleResultLimit caps the number of per-invocation rows scanned for a rule
// evaluation so a noisy contract cannot balloon a single evaluation.
const ruleResultLimit = 5000

// ---- postgres implementation ------------------------------------------------

func (s *postgresStore) CreateRule(ctx context.Context, r AlertRule) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO alert_rules (id, name, expression, contract_id, network, severity, enabled, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		r.ID, r.Name, r.Expression, r.ContractID, networkOrDefault(r.Network),
		r.Severity, r.Enabled, r.CreatedAt, r.UpdatedAt,
	)
	return err
}

func (s *postgresStore) UpdateRule(ctx context.Context, r AlertRule) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE alert_rules SET
			name        = $2,
			expression  = $3,
			contract_id = $4,
			severity    = $5,
			enabled     = $6,
			updated_at  = $7
		WHERE id = $1`,
		r.ID, r.Name, r.Expression, r.ContractID, r.Severity, r.Enabled, r.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *postgresStore) DeleteRule(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM alert_rules WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *postgresStore) GetRule(ctx context.Context, id string) (AlertRule, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, name, expression, contract_id, network, severity, enabled, created_at, updated_at
		FROM alert_rules WHERE id = $1`, id)
	return scanAlertRule(row)
}

func (s *postgresStore) ListRules(ctx context.Context, cursor string, limit int, enabled *bool, contractID string) ([]AlertRule, string, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	enabledFilter := "NULL"
	if enabled != nil {
		enabledFilter = "enabled"
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, expression, contract_id, network, severity, enabled, created_at, updated_at
		FROM alert_rules
		WHERE ($1 = '' OR id > $1)
		  AND ($2::boolean IS NULL OR enabled = $2)
		  AND ($3 = '' OR contract_id = $3)
		ORDER BY created_at DESC, id ASC
		LIMIT $4`, cursor, enabledFilter, contractID, limit+1)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var out []AlertRule
	for rows.Next() {
		var r AlertRule
		if err := rows.Scan(&r.ID, &r.Name, &r.Expression, &r.ContractID, &r.Network,
			&r.Severity, &r.Enabled, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, "", err
		}
		out = append(out, r)
	}
	if rows.Err() != nil {
		return nil, "", rows.Err()
	}
	var next string
	if len(out) > limit {
		next = out[limit-1].ID
		out = out[:limit]
	}
	return out, next, nil
}

func (s *postgresStore) RuleWindowStats(ctx context.Context, contractID string, window time.Duration) (rules.WindowStats, error) {
	stats := rules.WindowStats{Duration: window}

	rows, err := s.pool.Query(ctx, `
		SELECT status, resource_fee_charged, cpu_insn, mem_byte,
		       ledger_read_byte + ledger_write_byte AS ledger_bytes
		FROM invocations
		WHERE contract_id = $1
		  AND ledger_closed_at >= NOW() - make_interval(secs => $2)
		ORDER BY ledger_closed_at DESC
		LIMIT $3`, contractID, window.Seconds(), ruleResultLimit)
	if err != nil {
		return stats, err
	}
	defer rows.Close()

	for rows.Next() {
		var s rules.InvocationSample
		if err := rows.Scan(&s.Status, &s.FeeStroops, &s.CPUInsn, &s.MemBytes, &s.LedgerBytes); err != nil {
			return stats, err
		}
		stats.Invocations = append(stats.Invocations, s)
	}
	if rows.Err() != nil {
		return stats, rows.Err()
	}

	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM events
		WHERE contract_id = $1
		  AND ledger_closed_at >= NOW() - make_interval(secs => $2)`,
		contractID, window.Seconds(),
	).Scan(&stats.Events); err != nil {
		return stats, err
	}
	return stats, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAlertRule(row rowScanner) (AlertRule, error) {
	var r AlertRule
	err := row.Scan(&r.ID, &r.Name, &r.Expression, &r.ContractID, &r.Network,
		&r.Severity, &r.Enabled, &r.CreatedAt, &r.UpdatedAt)
	return r, err
}