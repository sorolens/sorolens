package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *postgresStore) CreateReportSubscription(ctx context.Context, sub ReportSubscription) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO report_subscriptions (id, email, frequency, day_of_week, created_at)
		VALUES ($1,$2,$3,$4,$5)`,
		sub.ID, sub.Email, sub.Frequency, sub.DayOfWeek, sub.CreatedAt,
	)
	return err
}

func (s *postgresStore) ListReportSubscriptions(ctx context.Context, email string) ([]ReportSubscription, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, email, frequency, day_of_week, created_at, unsubscribed_at
		FROM report_subscriptions
		WHERE email = $1 AND unsubscribed_at IS NULL
		ORDER BY created_at DESC`, email)
	if err != nil {
		return nil, fmt.Errorf("list report subscriptions: %w", err)
	}
	defer rows.Close()
	return scanReportSubscriptions(rows)
}

func (s *postgresStore) GetReportSubscription(ctx context.Context, id string) (ReportSubscription, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, email, frequency, day_of_week, created_at, unsubscribed_at
		FROM report_subscriptions
		WHERE id = $1`, id)
	var sub ReportSubscription
	err := row.Scan(&sub.ID, &sub.Email, &sub.Frequency, &sub.DayOfWeek, &sub.CreatedAt, &sub.UnsubscribedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ReportSubscription{}, ErrNotFound
	}
	return sub, err
}

func (s *postgresStore) DeleteReportSubscription(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE report_subscriptions SET unsubscribed_at = $2
		WHERE id = $1 AND unsubscribed_at IS NULL`, id, time.Now().UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *postgresStore) ListDueReportSubscriptions(ctx context.Context, frequency string, weekday int) ([]ReportSubscription, error) {
	// Daily digests are always due; weekly ones only on their chosen weekday.
	rows, err := s.pool.Query(ctx, `
		SELECT id, email, frequency, day_of_week, created_at, unsubscribed_at
		FROM report_subscriptions
		WHERE unsubscribed_at IS NULL
		  AND frequency = $1
		  AND ($1 = 'daily' OR day_of_week = $2)
		ORDER BY created_at ASC`, frequency, weekday)
	if err != nil {
		return nil, fmt.Errorf("list due report subscriptions: %w", err)
	}
	defer rows.Close()
	return scanReportSubscriptions(rows)
}

func scanReportSubscriptions(rows pgx.Rows) ([]ReportSubscription, error) {
	var out []ReportSubscription
	for rows.Next() {
		var sub ReportSubscription
		if err := rows.Scan(&sub.ID, &sub.Email, &sub.Frequency, &sub.DayOfWeek,
			&sub.CreatedAt, &sub.UnsubscribedAt); err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}
