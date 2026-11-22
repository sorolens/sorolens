package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func TestUsageStorePostgres(t *testing.T) {
	pool := pgPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "TRUNCATE TABLE api_keys CASCADE"); err != nil {
		t.Fatalf("truncate api_keys: %v", err)
	}
	s := store.NewFullStore(pool)

	created := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	key := store.APIKey{
		ID:        "key-usage-pg-1",
		Name:      "Usage Key",
		KeyPrefix: "sk_u",
		KeyHash:   store.HashKey("secret-usage"),
		Scopes:    []string{store.ScopeReadContracts},
		CreatedAt: created,
	}
	if err := s.CreateAPIKey(ctx, key); err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)
	yesterday := today.AddDate(0, 0, -1)

	records := []store.APIKeyUsageRecord{
		{
			KeyID:         key.ID,
			Date:          yesterday,
			EndpointGroup: "contracts",
			RequestCount:  15,
		},
		{
			KeyID:         key.ID,
			Date:          today,
			EndpointGroup: "contracts",
			RequestCount:  25,
		},
		{
			KeyID:         key.ID,
			Date:          today,
			EndpointGroup: "events",
			RequestCount:  10,
		},
	}

	if err := s.RecordBatchUsage(ctx, records); err != nil {
		t.Fatalf("RecordBatchUsage: %v", err)
	}

	// Record more usage for today to verify upsert increment
	additional := []store.APIKeyUsageRecord{
		{
			KeyID:         key.ID,
			Date:          today,
			EndpointGroup: "contracts",
			RequestCount:  5,
		},
	}
	if err := s.RecordBatchUsage(ctx, additional); err != nil {
		t.Fatalf("RecordBatchUsage upsert: %v", err)
	}

	// Verify monthly usage
	monthlyUsed, err := s.GetMonthlyKeyUsage(ctx, key.ID, today.Year(), today.Month())
	if err != nil {
		t.Fatalf("GetMonthlyKeyUsage: %v", err)
	}
	// 15 + (25 + 5) + 10 = 55
	if monthlyUsed != 55 {
		t.Fatalf("expected monthly used 55, got %d", monthlyUsed)
	}

	// Verify range lookup
	summary, err := s.GetKeyUsage(ctx, key.ID, yesterday, today.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("GetKeyUsage: %v", err)
	}
	if summary.TotalRequests != 55 {
		t.Errorf("expected total requests 55, got %d", summary.TotalRequests)
	}
	if summary.ByGroup["contracts"] != 45 {
		t.Errorf("expected contracts 45, got %d", summary.ByGroup["contracts"])
	}
	if summary.ByGroup["events"] != 10 {
		t.Errorf("expected events 10, got %d", summary.ByGroup["events"])
	}
}
