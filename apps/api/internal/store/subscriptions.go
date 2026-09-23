package store

import (
	"context"
	"fmt"
)

func (s *postgresStore) Create(ctx context.Context, sub AlertSubscription) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO alert_subscriptions
			(id, contract_id, webhook_url, severity_filter, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		sub.ID, sub.ContractID, sub.WebhookURL, sub.SeverityFilter,
		sub.CreatedAt, sub.UpdatedAt,
	)
	return err
}

func (s *postgresStore) ListByContract(ctx context.Context, contractID string) ([]AlertSubscription, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, contract_id, webhook_url, severity_filter, created_at, updated_at
		FROM alert_subscriptions WHERE contract_id = $1`, contractID)
	if err != nil {
		return nil, fmt.Errorf("list alert subscriptions by contract: %w", err)
	}
	defer rows.Close()

	var out []AlertSubscription
	for rows.Next() {
		var sub AlertSubscription
		if err := rows.Scan(&sub.ID, &sub.ContractID, &sub.WebhookURL,
			&sub.SeverityFilter, &sub.CreatedAt, &sub.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

func (s *postgresStore) Delete(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM alert_subscriptions WHERE id = $1`, id)
	return err
}

func (s *postgresStore) ListAll(ctx context.Context) ([]AlertSubscription, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, contract_id, webhook_url, severity_filter, created_at, updated_at
		FROM alert_subscriptions ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list all alert subscriptions: %w", err)
	}
	defer rows.Close()

	var out []AlertSubscription
	for rows.Next() {
		var sub AlertSubscription
		if err := rows.Scan(&sub.ID, &sub.ContractID, &sub.WebhookURL,
			&sub.SeverityFilter, &sub.CreatedAt, &sub.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}
