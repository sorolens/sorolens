package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// TestAPIKeyStorePostgres exercises every method of the real SQL
// implementation: creation with scope validation, hash lookup that hides
// revoked keys, atomic rotation, keyset pagination, revocation and last-used
// tracking.
func TestAPIKeyStorePostgres(t *testing.T) {
	pool := pgPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "TRUNCATE TABLE api_keys"); err != nil {
		t.Fatalf("truncate api_keys: %v", err)
	}
	s := store.NewFullStore(pool)

	created := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	keys := []store.APIKey{
		{ID: "key-a", Name: "Alpha", KeyPrefix: "sk_a", KeyHash: store.HashKey("secret-a"),
			Scopes: []string{store.ScopeReadContracts}, CreatedAt: created},
		{ID: "key-b", Name: "Beta", KeyPrefix: "sk_b", KeyHash: store.HashKey("secret-b"),
			Scopes: []string{store.ScopeReadWatchdog, store.ScopeWriteContracts}, CreatedAt: created.Add(time.Minute)},
		{ID: "key-c", Name: "Gamma", KeyPrefix: "sk_c", KeyHash: store.HashKey("secret-c"),
			Scopes: []string{store.ScopeAdmin}, CreatedAt: created.Add(2 * time.Minute)},
	}
	for _, k := range keys {
		if err := s.CreateAPIKey(ctx, k); err != nil {
			t.Fatalf("CreateAPIKey(%s): %v", k.ID, err)
		}
	}

	// Unknown scopes are rejected before the row reaches the database.
	if err := s.CreateAPIKey(ctx, store.APIKey{
		ID: "key-bad", Name: "Bad", KeyPrefix: "sk_x", KeyHash: store.HashKey("bad"),
		Scopes: []string{"read:everything"}, CreatedAt: created,
	}); !errors.Is(err, store.ErrInvalidScope) {
		t.Fatalf("CreateAPIKey with an unknown scope: want ErrInvalidScope, got %v", err)
	}

	got, err := s.GetAPIKeyByHash(ctx, store.HashKey("secret-a"))
	if err != nil {
		t.Fatalf("GetAPIKeyByHash: %v", err)
	}
	if got.ID != "key-a" || got.Name != "Alpha" || got.KeyPrefix != "sk_a" {
		t.Errorf("GetAPIKeyByHash = %+v", got)
	}
	if len(got.Scopes) != 1 || got.Scopes[0] != store.ScopeReadContracts {
		t.Errorf("scopes = %v, want [read:contracts]", got.Scopes)
	}
	if !got.CreatedAt.Equal(created) {
		t.Errorf("created_at = %s, want %s", got.CreatedAt, created)
	}
	if _, err := s.GetAPIKeyByHash(ctx, "no-such-hash"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetAPIKeyByHash(unknown): want ErrNotFound, got %v", err)
	}

	// Rotation invalidates the old secret immediately and keeps the identity.
	oldHash := store.HashKey("secret-a")
	newHash := store.HashKey("secret-a2")
	if err := s.RotateAPIKey(ctx, "key-a", newHash, "sk_a2"); err != nil {
		t.Fatalf("RotateAPIKey: %v", err)
	}
	if _, err := s.GetAPIKeyByHash(ctx, oldHash); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("old secret still authenticates after rotation: %v", err)
	}
	rotated, err := s.GetAPIKeyByHash(ctx, newHash)
	if err != nil {
		t.Fatalf("GetAPIKeyByHash(new secret): %v", err)
	}
	if rotated.ID != "key-a" || rotated.KeyPrefix != "sk_a2" || rotated.RotatedAt == nil {
		t.Errorf("rotated key = %+v, want same id, new prefix and rotated_at set", rotated)
	}
	if err := s.RotateAPIKey(ctx, "key-missing", newHash, "sk_x"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("RotateAPIKey(unknown): want ErrNotFound, got %v", err)
	}

	// Revocation hides the key from authentication but keeps it inspectable.
	if err := s.RevokeAPIKey(ctx, "key-b"); err != nil {
		t.Fatalf("RevokeAPIKey: %v", err)
	}
	if _, err := s.GetAPIKeyByHash(ctx, store.HashKey("secret-b")); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("revoked key still authenticates: %v", err)
	}
	revoked, err := s.GetAPIKey(ctx, "key-b")
	if err != nil {
		t.Fatalf("GetAPIKey(revoked): %v", err)
	}
	if revoked.RevokedAt == nil {
		t.Error("GetAPIKey(revoked) has no revoked_at")
	}
	if err := s.RevokeAPIKey(ctx, "key-b"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("second RevokeAPIKey: want ErrNotFound, got %v", err)
	}
	if err := s.RotateAPIKey(ctx, "key-b", "another-hash", "sk_b2"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("RotateAPIKey(revoked): want ErrNotFound, got %v", err)
	}

	// Touch records the last successful authentication.
	if err := s.TouchAPIKey(ctx, "key-c"); err != nil {
		t.Fatalf("TouchAPIKey: %v", err)
	}
	touched, err := s.GetAPIKey(ctx, "key-c")
	if err != nil {
		t.Fatalf("GetAPIKey after touch: %v", err)
	}
	if touched.LastUsedAt == nil {
		t.Error("TouchAPIKey did not set last_used_at")
	}

	// Keyset pagination by id ASC.
	page1, next, err := s.ListAPIKeys(ctx, "", 2)
	if err != nil {
		t.Fatalf("ListAPIKeys page 1: %v", err)
	}
	if len(page1) != 2 || page1[0].ID != "key-a" || page1[1].ID != "key-b" || next != "key-b" {
		t.Fatalf("page 1 = %v, next %q, want [key-a key-b] and key-b", idsOfKeys(page1), next)
	}
	page2, next2, err := s.ListAPIKeys(ctx, next, 2)
	if err != nil {
		t.Fatalf("ListAPIKeys page 2: %v", err)
	}
	if len(page2) != 1 || page2[0].ID != "key-c" || next2 != "" {
		t.Fatalf("page 2 = %v, next %q, want [key-c] and no cursor", idsOfKeys(page2), next2)
	}
}

func idsOfKeys(keys []store.APIKey) []string {
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = k.ID
	}
	return out
}
