package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

type mockUsageRedis struct {
	mu     sync.Mutex
	counts map[string]int64
	err    error
}

func newMockUsageRedis() *mockUsageRedis {
	return &mockUsageRedis{counts: make(map[string]int64)}
}

func (m *mockUsageRedis) Incr(_ context.Context, key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return 0, m.err
	}
	m.counts[key]++
	return m.counts[key], nil
}

func (m *mockUsageRedis) Expire(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return true, nil
}

func TestClassifyEndpointGroup(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/api/v1/contracts", "contracts"},
		{"/api/v1/contracts/CC123", "contracts"},
		{"/api/v1/contracts/CC123/events", "events"},
		{"/api/v1/contracts/CC123/invocations", "invocations"},
		{"/api/v1/events", "events"},
		{"/api/v1/invocations", "invocations"},
		{"/api/v1/alerts", "alerts"},
		{"/api/v1/subscriptions", "alerts"},
		{"/api/v1/admin/keys", "admin"},
		{"/api/v1/api-keys", "admin"},
		{"/api/v1/stats", "stats"},
		{"/api/v1/usage", "usage"},
		{"/health", "other"},
		{"/unknown/path", "other"},
	}

	for _, tt := range tests {
		got := ClassifyEndpointGroup(tt.path)
		if got != tt.want {
			t.Errorf("ClassifyEndpointGroup(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestUsageMeterQuotaEnforcement(t *testing.T) {
	redis := newMockUsageRedis()
	ms := store.NewMockStore()
	flusher := NewUsageFlusher(ms, time.Hour)

	quota := 3
	apiKey := store.APIKey{
		ID:           "key_quota_test",
		Name:         "Quota Key",
		MonthlyQuota: &quota,
	}

	handler := UsageMeter(redis, flusher)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	// Make 3 requests within quota
	for i := 1; i <= quota; i++ {
		req := httptest.NewRequest("GET", "/api/v1/contracts", nil)
		req = req.WithContext(WithIdentity(req.Context(), VerifiedIdentity{
			APIKey: &apiKey,
		}))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, rec.Code)
		}

		limitHdr := rec.Header().Get("X-RateLimit-Monthly-Quota")
		if limitHdr != strconv.Itoa(quota) {
			t.Errorf("request %d: expected quota header %d, got %s", i, quota, limitHdr)
		}
		remHdr := rec.Header().Get("X-RateLimit-Monthly-Remaining")
		expectedRem := strconv.Itoa(quota - i)
		if remHdr != expectedRem {
			t.Errorf("request %d: expected remaining %s, got %s", i, expectedRem, remHdr)
		}
	}

	// 4th request must exceed quota and return 429
	req := httptest.NewRequest("GET", "/api/v1/contracts", nil)
	req = req.WithContext(WithIdentity(req.Context(), VerifiedIdentity{
		APIKey: &apiKey,
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests, got %d", rec.Code)
	}

	if rec.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header to be set on 429")
	}

	var errBody struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&errBody); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}
	if errBody.Error.Code != "QUOTA_EXCEEDED" {
		t.Errorf("expected code QUOTA_EXCEEDED, got %s", errBody.Error.Code)
	}
}

func TestUsageMeterFailOpenOnRedisError(t *testing.T) {
	redis := &mockUsageRedis{err: errors.New("redis connection refused")}
	apiKey := store.APIKey{
		ID: "key_fail_open",
	}

	handler := UsageMeter(redis, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/api/v1/events", nil)
	req = req.WithContext(WithIdentity(req.Context(), VerifiedIdentity{
		APIKey: &apiKey,
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK on Redis failure (fail open), got %d", rec.Code)
	}
}

func TestUsageFlusherConcurrency(t *testing.T) {
	ms := store.NewMockStore()
	flusher := NewUsageFlusher(ms, time.Hour)

	var wg sync.WaitGroup
	requestsPerWorker := 100
	workers := 10

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < requestsPerWorker; j++ {
				flusher.Record("key_concurrent", "2026-11-20", "contracts")
			}
		}(i)
	}
	wg.Wait()

	if err := flusher.Flush(context.Background()); err != nil {
		t.Fatalf("flush error: %v", err)
	}

	summary, err := ms.GetKeyUsage(context.Background(), "key_concurrent",
		time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 11, 20, 23, 59, 59, 0, time.UTC),
	)
	// Key isn't registered yet so GetKeyUsage will return ErrNotFound
	if err != store.ErrNotFound {
		t.Logf("expected ErrNotFound for unregistered key, got %v", err)
	}

	// Register key and verify
	quota := 10000
	_ = ms.CreateAPIKey(context.Background(), store.APIKey{
		ID:           "key_concurrent",
		Name:         "Concurrent Key",
		MonthlyQuota: &quota,
		Scopes:       []string{store.ScopeReadContracts},
	})

	summary, err = ms.GetKeyUsage(context.Background(), "key_concurrent",
		time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 11, 20, 23, 59, 59, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("GetKeyUsage: %v", err)
	}

	expectedTotal := int64(workers * requestsPerWorker)
	if summary.TotalRequests != expectedTotal {
		t.Errorf("TotalRequests = %d, want %d", summary.TotalRequests, expectedTotal)
	}
	if summary.ByGroup["contracts"] != expectedTotal {
		t.Errorf("ByGroup['contracts'] = %d, want %d", summary.ByGroup["contracts"], expectedTotal)
	}
}
