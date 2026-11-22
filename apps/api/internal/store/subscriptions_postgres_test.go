package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// TestSubscriptionStorePostgres covers the real SQL behind subscription
// reads, the global list and signing-secret rotation.
func TestSubscriptionStorePostgres(t *testing.T) {
	pool := pgPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "TRUNCATE TABLE alert_subscriptions, monitored_contracts CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	s := store.NewFullStore(pool)

	registered := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	for _, id := range []string{"CSUB-A", "CSUB-B"} {
		if err := s.UpsertMonitoredContract(ctx, store.MonitoredContract{
			ContractID: id, Network: "testnet", Name: id, Owner: "GOWNER",
			Status: "Healthy", RegisteredAt: registered,
		}); err != nil {
			t.Fatalf("UpsertMonitoredContract(%s): %v", id, err)
		}
	}

	created := time.Date(2026, 2, 2, 10, 0, 0, 0, time.UTC)
	subA := store.AlertSubscription{
		ID: "sub-a", ContractID: "CSUB-A", WebhookURL: "https://hooks.example/a",
		SeverityFilter: "Warning", ChannelType: "slack", RoutingKey: "rk-a",
		SigningSecret: "whsec_one", SigningSecretHash: "hash-one",
		SigningSecretCreatedAt: created, CreatedAt: created, UpdatedAt: created,
	}
	subB := store.AlertSubscription{
		ID: "sub-b", ContractID: "CSUB-B", WebhookURL: "https://hooks.example/b",
		SeverityFilter: "Critical",
		SigningSecret:  "whsec_two", SigningSecretHash: "hash-two",
		SigningSecretCreatedAt: created, CreatedAt: created.Add(time.Hour), UpdatedAt: created.Add(time.Hour),
	}
	for _, sub := range []store.AlertSubscription{subA, subB} {
		if err := s.Create(ctx, sub); err != nil {
			t.Fatalf("Create(%s): %v", sub.ID, err)
		}
	}

	got, err := s.GetSubscription(ctx, "sub-a")
	if err != nil {
		t.Fatalf("GetSubscription: %v", err)
	}
	if got.ContractID != "CSUB-A" || got.WebhookURL != subA.WebhookURL ||
		got.SeverityFilter != "Warning" || got.ChannelType != "slack" || got.RoutingKey != "rk-a" {
		t.Errorf("GetSubscription = %+v", got)
	}
	if got.SigningSecret != "whsec_one" || got.SigningSecretHash != "hash-one" {
		t.Errorf("signing secret columns = %q / %q", got.SigningSecret, got.SigningSecretHash)
	}
	if !got.SigningSecretCreatedAt.Equal(created) || got.SigningSecretRotatedAt != nil {
		t.Errorf("signing secret timestamps = %s / %v", got.SigningSecretCreatedAt, got.SigningSecretRotatedAt)
	}
	if _, err := s.GetSubscription(ctx, "sub-missing"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetSubscription(unknown): want ErrNotFound, got %v", err)
	}

	byContract, err := s.ListByContract(ctx, "CSUB-A")
	if err != nil {
		t.Fatalf("ListByContract: %v", err)
	}
	if len(byContract) != 1 || byContract[0].ID != "sub-a" {
		t.Errorf("ListByContract = %+v", byContract)
	}

	all, err := s.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 2 || all[0].ID != "sub-b" || all[1].ID != "sub-a" {
		t.Errorf("ListAll order = %v, want newest created_at first", subscriptionIDs(all))
	}

	rotatedAt := created.Add(48 * time.Hour)
	if err := s.RotateSigningSecret(ctx, "sub-a", "whsec_rotated", "hash-rotated", rotatedAt); err != nil {
		t.Fatalf("RotateSigningSecret: %v", err)
	}
	rotated, err := s.GetSubscription(ctx, "sub-a")
	if err != nil {
		t.Fatalf("GetSubscription after rotate: %v", err)
	}
	if rotated.SigningSecret != "whsec_rotated" || rotated.SigningSecretHash != "hash-rotated" {
		t.Errorf("rotated secret = %q / %q", rotated.SigningSecret, rotated.SigningSecretHash)
	}
	if rotated.SigningSecretRotatedAt == nil || !rotated.SigningSecretRotatedAt.Equal(rotatedAt) {
		t.Errorf("rotated_at = %v, want %s", rotated.SigningSecretRotatedAt, rotatedAt)
	}
	if !rotated.UpdatedAt.Equal(rotatedAt) {
		t.Errorf("updated_at = %s, want %s", rotated.UpdatedAt, rotatedAt)
	}
	if err := s.RotateSigningSecret(ctx, "sub-missing", "x", "y", rotatedAt); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("RotateSigningSecret(unknown): want ErrNotFound, got %v", err)
	}
	if err := s.Delete(ctx, "sub-missing"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Delete(unknown): want ErrNotFound, got %v", err)
	}

	// Deleting the monitored contract cascades to its subscriptions.
	if err := s.DeleteMonitoredContract(ctx, "CSUB-A"); err != nil {
		t.Fatalf("DeleteMonitoredContract: %v", err)
	}
	if _, err := s.GetSubscription(ctx, "sub-a"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("subscription survived its contract deletion: %v", err)
	}
}

func subscriptionIDs(subs []store.AlertSubscription) []string {
	out := make([]string, len(subs))
	for i, s := range subs {
		out[i] = s.ID
	}
	return out
}
