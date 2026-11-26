package store

import (
	"context"
	"encoding/json"
	"time"
)

// PushSubscription is a browser Web Push endpoint owned by a workspace user
// (issue #592). Keys are the subscription's auth and p256dh values; Filters
// holds the optional delivery filter (severity, contract_id, network).
//
// The endpoint is the natural key: re-subscribing the same browser updates the
// existing row instead of accumulating duplicates.
type PushSubscription struct {
	ID        string
	Endpoint  string
	AuthKey   string
	P256DHKey string
	UserID    string
	Filters   map[string]any
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PushSubscriptionStore is the persistence surface for Web Push subscriptions.
// Passing an empty userID to List or Delete means "every user"; that form is
// reserved for the internal dispatcher, which authenticates with the shared
// dispatch secret rather than a user session.
type PushSubscriptionStore interface {
	UpsertPushSubscription(ctx context.Context, sub PushSubscription) (bool, error)
	ListPushSubscriptions(ctx context.Context, userID string) ([]PushSubscription, error)
	DeletePushSubscription(ctx context.Context, userID, endpoint string) (bool, error)
}

// UpsertPushSubscription inserts the subscription, or refreshes an existing row
// for the same endpoint only when the caller already owns it. It returns false
// when the endpoint belongs to a different user, so one account can never take
// over another account's endpoint or keys.
func (s *postgresStore) UpsertPushSubscription(ctx context.Context, sub PushSubscription) (bool, error) {
	filters := sub.Filters
	if filters == nil {
		filters = map[string]any{}
	}
	raw, err := json.Marshal(filters)
	if err != nil {
		return false, err
	}
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO push_subscriptions
			(id, endpoint, auth_key, p256dh_key, user_id, filters, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		ON CONFLICT (endpoint) DO UPDATE SET
			auth_key    = EXCLUDED.auth_key,
			p256dh_key  = EXCLUDED.p256dh_key,
			filters     = EXCLUDED.filters,
			updated_at  = NOW()
		WHERE push_subscriptions.user_id = EXCLUDED.user_id`,
		sub.ID, sub.Endpoint, sub.AuthKey, sub.P256DHKey, sub.UserID, raw)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s *postgresStore) ListPushSubscriptions(ctx context.Context, userID string) ([]PushSubscription, error) {
	query := `SELECT id, endpoint, auth_key, p256dh_key, user_id, filters, created_at, updated_at
		FROM push_subscriptions`
	args := []any{}
	if userID != "" {
		query += ` WHERE user_id = $1`
		args = append(args, userID)
	}
	query += ` ORDER BY created_at ASC`

	rows, err := s.readDB(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []PushSubscription
	for rows.Next() {
		var (
			sub     PushSubscription
			raw     []byte
			created time.Time
			updated time.Time
		)
		if err := rows.Scan(&sub.ID, &sub.Endpoint, &sub.AuthKey, &sub.P256DHKey, &sub.UserID, &raw, &created, &updated); err != nil {
			return nil, err
		}
		sub.Filters = map[string]any{}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &sub.Filters)
		}
		sub.CreatedAt, sub.UpdatedAt = created, updated
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

// DeletePushSubscription removes one endpoint, scoped to its owner unless
// userID is empty (internal dispatch). It reports whether a row was deleted.
func (s *postgresStore) DeletePushSubscription(ctx context.Context, userID, endpoint string) (bool, error) {
	query := `DELETE FROM push_subscriptions WHERE endpoint = $1`
	args := []any{endpoint}
	if userID != "" {
		query += ` AND user_id = $2`
		args = append(args, userID)
	}
	tag, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ensure the postgres store still satisfies the interface at compile time.
var _ PushSubscriptionStore = (*postgresStore)(nil)
