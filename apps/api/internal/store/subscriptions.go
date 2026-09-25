package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *postgresStore) Create(ctx context.Context, sub AlertSubscription) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO alert_subscriptions
			(id, contract_id, webhook_url, severity_filter, last_delivery_status, last_delivery_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		sub.ID, sub.ContractID, sub.WebhookURL, sub.SeverityFilter,
		sub.LastDeliveryStatus, sub.LastDeliveryAt,
		sub.CreatedAt, sub.UpdatedAt,
	)
	return err
}

func (s *postgresStore) GetByID(ctx context.Context, id string) (AlertSubscription, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, contract_id, webhook_url, severity_filter, last_delivery_status, last_delivery_at, created_at, updated_at
		FROM alert_subscriptions WHERE id = $1`, id)
	var sub AlertSubscription
	err := row.Scan(&sub.ID, &sub.ContractID, &sub.WebhookURL, &sub.SeverityFilter,
		&sub.LastDeliveryStatus, &sub.LastDeliveryAt, &sub.CreatedAt, &sub.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AlertSubscription{}, ErrNotFound
		}
		return AlertSubscription{}, fmt.Errorf("get alert subscription by id: %w", err)
	}
	return sub, nil
}

func (s *postgresStore) ListByContract(ctx context.Context, contractID string) ([]AlertSubscription, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, contract_id, webhook_url, severity_filter, last_delivery_status, last_delivery_at, created_at, updated_at
		FROM alert_subscriptions WHERE contract_id = $1`, contractID)
	if err != nil {
		return nil, fmt.Errorf("list alert subscriptions by contract: %w", err)
	}
	defer rows.Close()

	var out []AlertSubscription
	for rows.Next() {
		var sub AlertSubscription
		if err := rows.Scan(&sub.ID, &sub.ContractID, &sub.WebhookURL,
			&sub.SeverityFilter, &sub.LastDeliveryStatus, &sub.LastDeliveryAt,
			&sub.CreatedAt, &sub.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

func (s *postgresStore) Delete(ctx context.Context, id string) error {
	res, err := s.pool.Exec(ctx, `DELETE FROM alert_subscriptions WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *postgresStore) ListAll(ctx context.Context) ([]AlertSubscription, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, contract_id, webhook_url, severity_filter, last_delivery_status, last_delivery_at, created_at, updated_at
		FROM alert_subscriptions ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list all alert subscriptions: %w", err)
	}
	defer rows.Close()

	var out []AlertSubscription
	for rows.Next() {
		var sub AlertSubscription
		if err := rows.Scan(&sub.ID, &sub.ContractID, &sub.WebhookURL,
			&sub.SeverityFilter, &sub.LastDeliveryStatus, &sub.LastDeliveryAt,
			&sub.CreatedAt, &sub.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

func (s *postgresStore) UpdateDeliveryStatus(ctx context.Context, subscriptionID string, status string, at time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE alert_subscriptions
		SET last_delivery_status = $1, last_delivery_at = $2, updated_at = NOW()
		WHERE id = $3`,
		status, at, subscriptionID,
	)
	return err
}

func (s *postgresStore) CreateDelivery(ctx context.Context, d WebhookDelivery) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO webhook_deliveries
			(id, subscription_id, payload, status, attempt, max_attempts, next_attempt_at, response_code, error_message, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		d.ID, d.SubscriptionID, d.Payload, d.Status, d.Attempt, d.MaxAttempts, d.NextAttemptAt, d.ResponseCode, d.ErrorMessage, d.CreatedAt, d.UpdatedAt,
	)
	return err
}

func (s *postgresStore) UpdateDelivery(ctx context.Context, d WebhookDelivery) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE webhook_deliveries
		SET status = $1, attempt = $2, max_attempts = $3, next_attempt_at = $4, response_code = $5, error_message = $6, updated_at = $7
		WHERE id = $8`,
		d.Status, d.Attempt, d.MaxAttempts, d.NextAttemptAt, d.ResponseCode, d.ErrorMessage, d.UpdatedAt, d.ID,
	)
	return err
}

func (s *postgresStore) ListDeliveriesBySubscription(ctx context.Context, subscriptionID string, page int, limit int) ([]WebhookDelivery, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}
	offset := (page - 1) * limit

	var total int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM webhook_deliveries WHERE subscription_id = $1`, subscriptionID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count webhook deliveries: %w", err)
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, subscription_id, payload, status, attempt, max_attempts, next_attempt_at, response_code, error_message, created_at, updated_at
		FROM webhook_deliveries
		WHERE subscription_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, subscriptionID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list webhook deliveries: %w", err)
	}
	defer rows.Close()

	var out []WebhookDelivery
	for rows.Next() {
		var d WebhookDelivery
		var errMsg *string
		if err := rows.Scan(&d.ID, &d.SubscriptionID, &d.Payload, &d.Status, &d.Attempt, &d.MaxAttempts, &d.NextAttemptAt, &d.ResponseCode, &errMsg, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, 0, err
		}
		if errMsg != nil {
			d.ErrorMessage = *errMsg
		}
		out = append(out, d)
	}
	return out, total, rows.Err()
}

func (s *postgresStore) GetPendingDeliveries(ctx context.Context, limit int) ([]WebhookDelivery, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, subscription_id, payload, status, attempt, max_attempts, next_attempt_at, response_code, error_message, created_at, updated_at
		FROM webhook_deliveries
		WHERE status = 'pending' AND next_attempt_at <= NOW()
		ORDER BY next_attempt_at ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("get pending deliveries: %w", err)
	}
	defer rows.Close()

	var out []WebhookDelivery
	for rows.Next() {
		var d WebhookDelivery
		var errMsg *string
		if err := rows.Scan(&d.ID, &d.SubscriptionID, &d.Payload, &d.Status, &d.Attempt, &d.MaxAttempts, &d.NextAttemptAt, &d.ResponseCode, &errMsg, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		if errMsg != nil {
			d.ErrorMessage = *errMsg
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
