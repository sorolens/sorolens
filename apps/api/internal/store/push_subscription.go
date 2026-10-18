package store

import (
	"context"
	"time"
)

// PushSubscription is a browser Web Push subscription (issue #592).
//
// Endpoint is the push-service URL the browser reports; it is globally
// unique and effectively a bearer secret for that browser's channel, so it
// must never appear in logs. P256dhKey is the subscriber's ECDH public key
// and AuthKey the RFC 8291 auth secret — both are only ever handed to the
// authenticated surface that needs them for sending.
//
// Filters carries the subscriber's preferences as raw JSONB (contract ids,
// severities, ...) so alert fan-out can narrow delivery server-side.
type PushSubscription struct {
	ID        string
	Endpoint  string
	P256dhKey string
	AuthKey   string
	// UserID is the optional owner binding ("" = anonymous browser
	// subscription, mirroring the self-service report subscriptions).
	UserID    string
	Filters   []byte
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PushSubscriptionStore is the data-access surface for Web Push
// subscriptions. Deleting is a hard delete: a subscription whose browser is
// gone (HTTP 410 during a send) has no audit value, unlike a report digest.
type PushSubscriptionStore interface {
	// UpsertPushSubscription inserts or updates by endpoint. When a row with
	// the same endpoint already exists its id/created_at are preserved and
	// the keys, user binding and filters are refreshed — re-subscribing is
	// therefore idempotent. It returns the persisted row and whether the
	// call created it (created=false means an update in place).
	UpsertPushSubscription(ctx context.Context, s PushSubscription) (PushSubscription, bool, error)

	// GetPushSubscription returns one subscription by id, or ErrNotFound.
	GetPushSubscription(ctx context.Context, id string) (PushSubscription, error)

	// ListPushSubscriptions returns every subscription, or (when userID is
	// non-empty) only that user's. Used by the alert fan-out and by the
	// management UI, both of which sit behind authenticated scopes.
	ListPushSubscriptions(ctx context.Context, userID string) ([]PushSubscription, error)

	// DeletePushSubscription removes a subscription by id. Returns
	// ErrNotFound when nothing was removed.
	DeletePushSubscription(ctx context.Context, id string) error

	// DeletePushSubscriptionByEndpoint removes a subscription by its push
	// endpoint (the shape the browser-side unsubscribe flow and 410-Gone
	// cleanup use). Returns ErrNotFound when nothing was removed.
	DeletePushSubscriptionByEndpoint(ctx context.Context, endpoint string) error
}
