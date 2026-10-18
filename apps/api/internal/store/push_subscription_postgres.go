package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// UpsertPushSubscription implements PushSubscriptionStore.
//
// The INSERT ... ON CONFLICT ... DO UPDATE ... RETURNING round-trips once:
// (xmax = 0) reports whether this statement created the row (true) or
// refreshed an existing one (false).
func (s *postgresStore) UpsertPushSubscription(ctx context.Context, sub PushSubscription) (PushSubscription, bool, error) {
	filters := sub.Filters
	if len(filters) == 0 {
		filters = []byte("{}")
	}
	var userID any
	if sub.UserID != "" {
		userID = sub.UserID
	}
	now := time.Now().UTC()

	var out PushSubscription
	var created bool
	err := s.pool.QueryRow(ctx, `
		INSERT INTO push_subscriptions
		    (id, endpoint, p256dh_key, auth_key, user_id, filters, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7,$7)
		ON CONFLICT (endpoint) DO UPDATE SET
		    p256dh_key = EXCLUDED.p256dh_key,
		    auth_key   = EXCLUDED.auth_key,
		    user_id    = EXCLUDED.user_id,
		    filters    = EXCLUDED.filters,
		    updated_at = EXCLUDED.updated_at
		RETURNING id, endpoint, p256dh_key, auth_key, COALESCE(user_id, ''),
		          filters, created_at, updated_at, (xmax = 0) AS created`,
		sub.ID, sub.Endpoint, sub.P256dhKey, sub.AuthKey, userID, string(filters), now,
	).Scan(&out.ID, &out.Endpoint, &out.P256dhKey, &out.AuthKey, &out.UserID,
		&out.Filters, &out.CreatedAt, &out.UpdatedAt, &created)
	if err != nil {
		return PushSubscription{}, false, fmt.Errorf("upsert push subscription: %w", err)
	}
	return out, created, nil
}

func (s *postgresStore) GetPushSubscription(ctx context.Context, id string) (PushSubscription, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, endpoint, p256dh_key, auth_key, COALESCE(user_id, ''),
		       filters, created_at, updated_at
		FROM push_subscriptions
		WHERE id = $1`, id)
	var sub PushSubscription
	err := row.Scan(&sub.ID, &sub.Endpoint, &sub.P256dhKey, &sub.AuthKey,
		&sub.UserID, &sub.Filters, &sub.CreatedAt, &sub.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return PushSubscription{}, ErrNotFound
	}
	return sub, err
}

func (s *postgresStore) ListPushSubscriptions(ctx context.Context, userID string) ([]PushSubscription, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, endpoint, p256dh_key, auth_key, COALESCE(user_id, ''),
		       filters, created_at, updated_at
		FROM push_subscriptions
		WHERE $1 = '' OR user_id = $1
		ORDER BY created_at ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list push subscriptions: %w", err)
	}
	defer rows.Close()

	var out []PushSubscription
	for rows.Next() {
		var sub PushSubscription
		if err := rows.Scan(&sub.ID, &sub.Endpoint, &sub.P256dhKey, &sub.AuthKey,
			&sub.UserID, &sub.Filters, &sub.CreatedAt, &sub.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

func (s *postgresStore) DeletePushSubscription(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM push_subscriptions WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *postgresStore) DeletePushSubscriptionByEndpoint(ctx context.Context, endpoint string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM push_subscriptions WHERE endpoint = $1`, endpoint)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
