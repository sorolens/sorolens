package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *postgresStore) CreateEscalationPolicy(ctx context.Context, p EscalationPolicy) (EscalationPolicy, error) {
	if p.Name == "" {
		return EscalationPolicy{}, fmt.Errorf("%w: name is required", ErrInvalidEscalationPolicy)
	}
	steps, err := MarshalSteps(p.Steps)
	if err != nil {
		return EscalationPolicy{}, fmt.Errorf("escalation policy marshal steps: %w", err)
	}
	now := time.Now().UTC()
	var id string
	err = s.pool.QueryRow(ctx, `
		INSERT INTO escalation_policies (name, steps, created_at, updated_at)
		VALUES ($1, $2, $3, $3)
		RETURNING id`,
		p.Name, steps, now,
	).Scan(&id)
	if err != nil {
		return EscalationPolicy{}, fmt.Errorf("create escalation policy: %w", err)
	}
	p.ID = id
	p.CreatedAt = now
	p.UpdatedAt = now
	return p, nil
}

func (s *postgresStore) GetEscalationPolicy(ctx context.Context, id string) (EscalationPolicy, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, name, steps, created_at, updated_at
		FROM escalation_policies
		WHERE id = $1`, id)
	return scanEscalationPolicy(row)
}

func (s *postgresStore) ListEscalationPolicies(ctx context.Context) ([]EscalationPolicy, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, steps, created_at, updated_at
		FROM escalation_policies
		ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list escalation policies: %w", err)
	}
	defer rows.Close()
	var out []EscalationPolicy
	for rows.Next() {
		p, err := scanEscalationPolicy(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *postgresStore) UpdateEscalationPolicy(ctx context.Context, p EscalationPolicy) (EscalationPolicy, error) {
	if p.ID == "" {
		return EscalationPolicy{}, fmt.Errorf("%w: id is required", ErrInvalidEscalationPolicy)
	}
	steps, err := MarshalSteps(p.Steps)
	if err != nil {
		return EscalationPolicy{}, fmt.Errorf("escalation policy marshal steps: %w", err)
	}
	now := time.Now().UTC()
	var out EscalationPolicy
	err = s.pool.QueryRow(ctx, `
		UPDATE escalation_policies
		SET name = $2, steps = $3, updated_at = $4
		WHERE id = $1
		RETURNING id, name, steps, created_at, updated_at`,
		p.ID, p.Name, steps, now,
	).Scan(&out.ID, &out.Name, &steps, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return EscalationPolicy{}, ErrEscalationPolicyNotFound
	}
	if err != nil {
		return EscalationPolicy{}, fmt.Errorf("update escalation policy: %w", err)
	}
	out.Steps, _ = UnmarshalSteps(steps)
	return out, nil
}

func (s *postgresStore) DeleteEscalationPolicy(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM escalation_policies WHERE id = $1`, id)
	return err
}

type policyScanner interface {
	Scan(dest ...any) error
}

func scanEscalationPolicy(row policyScanner) (EscalationPolicy, error) {
	var p EscalationPolicy
	var stepsJSON []byte
	err := row.Scan(&p.ID, &p.Name, &stepsJSON, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return EscalationPolicy{}, ErrEscalationPolicyNotFound
	}
	if err != nil {
		return EscalationPolicy{}, fmt.Errorf("scan escalation policy: %w", err)
	}
	p.Steps, _ = UnmarshalSteps(stepsJSON)
	return p, nil
}
