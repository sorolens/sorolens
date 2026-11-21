package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/middleware"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func TestGetUsage(t *testing.T) {
	ms := store.NewMockStore()
	quota := 1000
	apiKey := store.APIKey{
		ID:           "test_key_1",
		Name:         "App Key",
		Scopes:       []string{store.ScopeReadContracts},
		MonthlyQuota: &quota,
	}
	_ = ms.CreateAPIKey(context.Background(), apiKey)

	adminKey := store.APIKey{
		ID:     "admin_key_1",
		Name:   "Admin Key",
		Scopes: []string{store.ScopeAdmin},
	}
	_ = ms.CreateAPIKey(context.Background(), adminKey)

	// Record usage in store
	today := time.Now().UTC()
	yesterday := today.AddDate(0, 0, -1)
	_ = ms.RecordBatchUsage(context.Background(), []store.APIKeyUsageRecord{
		{
			KeyID:         "test_key_1",
			Date:          yesterday,
			EndpointGroup: "contracts",
			RequestCount:  25,
		},
		{
			KeyID:         "test_key_1",
			Date:          today,
			EndpointGroup: "events",
			RequestCount:  15,
		},
		{
			KeyID:         "test_key_1",
			Date:          today,
			EndpointGroup: "contracts",
			RequestCount:  10,
		},
	})

	r := newTestHandler(ms, true, true)

	t.Run("unauthenticated request returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/usage", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("authenticated key reads its own usage", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/usage", nil)
		req = req.WithContext(middleware.WithIdentity(req.Context(), middleware.VerifiedIdentity{
			APIKey: &apiKey,
		}))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		var summary store.KeyUsageSummary
		if err := json.NewDecoder(rec.Body).Decode(&summary); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if summary.KeyID != "test_key_1" {
			t.Errorf("KeyID = %s, want test_key_1", summary.KeyID)
		}
		if summary.TotalRequests != 50 {
			t.Errorf("TotalRequests = %d, want 50", summary.TotalRequests)
		}
		if summary.ByGroup["contracts"] != 35 {
			t.Errorf("ByGroup['contracts'] = %d, want 35", summary.ByGroup["contracts"])
		}
		if summary.ByGroup["events"] != 15 {
			t.Errorf("ByGroup['events'] = %d, want 15", summary.ByGroup["events"])
		}
		if summary.MonthlyQuota == nil || *summary.MonthlyQuota != 1000 {
			t.Errorf("MonthlyQuota = %v, want 1000", summary.MonthlyQuota)
		}
		if summary.MonthUsed != 50 {
			t.Errorf("MonthUsed = %d, want 50", summary.MonthUsed)
		}
		if summary.MonthRemaining == nil || *summary.MonthRemaining != 950 {
			t.Errorf("MonthRemaining = %v, want 950", summary.MonthRemaining)
		}
	})

	t.Run("non-admin key cannot read another key's usage", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/usage?key_id=admin_key_1", nil)
		req = req.WithContext(middleware.WithIdentity(req.Context(), middleware.VerifiedIdentity{
			APIKey: &apiKey,
		}))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got %d", rec.Code)
		}
	})

	t.Run("admin key can read any key's usage", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/usage?key_id=test_key_1", nil)
		req = req.WithContext(middleware.WithIdentity(req.Context(), middleware.VerifiedIdentity{
			APIKey: &adminKey,
		}))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
	})

	t.Run("query non-existent key returns 404", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/usage?key_id=does_not_exist", nil)
		req = req.WithContext(middleware.WithIdentity(req.Context(), middleware.VerifiedIdentity{
			APIKey: &adminKey,
		}))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", rec.Code)
		}
	})

	t.Run("date filtering query parameters", func(t *testing.T) {
		fromStr := today.Format("2006-01-02")
		req := httptest.NewRequest("GET", "/api/v1/usage?from="+fromStr, nil)
		req = req.WithContext(middleware.WithIdentity(req.Context(), middleware.VerifiedIdentity{
			APIKey: &apiKey,
		}))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var summary store.KeyUsageSummary
		if err := json.NewDecoder(rec.Body).Decode(&summary); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Only today's 25 requests should match
		if summary.TotalRequests != 25 {
			t.Errorf("TotalRequests = %d, want 25", summary.TotalRequests)
		}
	})
}
