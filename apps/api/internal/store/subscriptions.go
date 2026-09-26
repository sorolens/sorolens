package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// subscriptionColumns is the projection shared by every read so the scan order
// stays in one place.
const subscriptionColumns = `id, contract_id, webhook_url, severity_filter,
	signing_secret, signing_secret_hash, signing_secret_created_at, signing_secret_rotated_at,
	created_at, updated_at`

func scanSubscription(row pgx.Row) (AlertSubscription, error) {
	var sub AlertSubscription
	err := row.Scan(
		&sub.ID, &sub.ContractID, &sub.WebhookURL, &sub.SeverityFilter,
		&sub.SigningSecret, &sub.SigningSecretHash, &sub.SigningSecretCreatedAt, &sub.SigningSecretRotatedAt,
		&sub.CreatedAt, &sub.UpdatedAt,
	)
	return sub, err
}

func (s *postgresStore) Create(ctx context.Context, sub AlertSubscription) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO alert_subscriptions
			(id, contract_id, webhook_url, severity_filter,
			 signing_secret, signing_secret_hash, signing_secret_created_at, signing_secret_rotated_at,
			 created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		sub.ID, sub.ContractID, sub.WebhookURL, sub.SeverityFilter,
		sub.SigningSecret, sub.SigningSecretHash, sub.SigningSecretCreatedAt, sub.SigningSecretRotatedAt,
		sub.CreatedAt, sub.UpdatedAt,
	)
	return err
}

func (s *postgresStore) ListByContract(ctx context.Context, contractID string) ([]AlertSubscription, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+subscriptionColumns+`
		FROM alert_subscriptions WHERE contract_id = $1`, contractID)
	if err != nil {
		return nil, fmt.Errorf("list alert subscriptions by contract: %w", err)
	}
	defer rows.Close()

	var out []AlertSubscription
	for rows.Next() {
		sub, err := scanSubscription(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

func (s *postgresStore) GetSubscription(ctx context.Context, id string) (AlertSubscription, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+subscriptionColumns+`
		FROM alert_subscriptions WHERE id = $1`, id)
	sub, err := scanSubscription(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return AlertSubscription{}, ErrNotFound
	}
	if err != nil {
		return AlertSubscription{}, fmt.Errorf("get alert subscription: %w", err)
	}
	return sub, nil
}

func (s *postgresStore) RotateSigningSecret(ctx context.Context, id, secret, hash string, rotatedAt time.Time) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE alert_subscriptions
		SET signing_secret = $2,
		    signing_secret_hash = $3,
		    signing_secret_rotated_at = $4,
		    updated_at = $4
		WHERE id = $1`,
		id, secret, hash, rotatedAt,
	)
	if err != nil {
		return fmt.Errorf("rotate signing secret: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *postgresStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM alert_subscriptions WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *postgresStore) ListAll(ctx context.Context) ([]AlertSubscription, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+subscriptionColumns+`
		FROM alert_subscriptions ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list all alert subscriptions: %w", err)
	}
	defer rows.Close()

	var out []AlertSubscription
	for rows.Next() {
		sub, err := scanSubscription(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}
