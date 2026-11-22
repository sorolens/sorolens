package handler_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/config"
	"github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/router"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

type requireAuthRedisClient struct{}

func (m *requireAuthRedisClient) Incr(ctx context.Context, key string) (int64, error) { return 1, nil }
func (m *requireAuthRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	return true, nil
}

func newRequireAuthTestRouter(ms *store.MockStore, requireAuth bool) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := &handler.Handler{
		Store:       ms,
		DB:          &store.MockPinger{Healthy: true},
		Redis:       &store.MockPinger{Healthy: true},
		RedisClient: &requireAuthRedisClient{},
		Logger:      logger,
		RequireAuth: requireAuth,
	}
	return router.New(h, config.DefaultRequestMaxBodyBytes)
}

func TestRequireAuthMode(t *testing.T) {
	ms := store.NewMockStore()
	validToken := "sl_valid_token_test"
	ms.AddAPIKey(store.APIKey{
		ID:        "key-reader",
		Name:      "reader",
		KeyPrefix: "sl_valid",
		KeyHash:   store.HashKey(validToken),
		Scopes:    []string{store.ScopeReadContracts},
		CreatedAt: time.Now().UTC(),
	})

	srv := newRequireAuthTestRouter(ms, true)

	// Health endpoints must pass without authentication
	for _, path := range []string{"/health", "/readyz", "/api/v1/health"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("GET %s in require-auth mode: want 200, got %d", path, w.Code)
		}
	}

	// Normal endpoints without auth should be rejected with 401
	reqUnauth := httptest.NewRequest(http.MethodGet, "/api/v1/stats/global", nil)
	wUnauth := httptest.NewRecorder()
	srv.ServeHTTP(wUnauth, reqUnauth)
	if wUnauth.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated GET in require-auth mode: want 401, got %d", wUnauth.Code)
	}

	// With valid auth, normal endpoints succeed
	reqAuth := httptest.NewRequest(http.MethodGet, "/api/v1/stats/global", nil)
	reqAuth.Header.Set("Authorization", "Bearer "+validToken)
	wAuth := httptest.NewRecorder()
	srv.ServeHTTP(wAuth, reqAuth)
	if wAuth.Code != http.StatusOK {
		t.Errorf("authenticated GET in require-auth mode: want 200, got %d", wAuth.Code)
	}
}

func TestAPIKeyExpiryEnforcement(t *testing.T) {
	ms := store.NewMockStore()
	expiredToken := "sl_expired_token_test"
	past := time.Now().Add(-1 * time.Hour).UTC()
	ms.AddAPIKey(store.APIKey{
		ID:        "key-expired",
		Name:      "expired",
		KeyPrefix: "sl_expir",
		KeyHash:   store.HashKey(expiredToken),
		Scopes:    []string{store.ScopeReadContracts},
		CreatedAt: time.Now().Add(-2 * time.Hour).UTC(),
		ExpiresAt: &past,
	})

	futureToken := "sl_future_token_test"
	future := time.Now().Add(1 * time.Hour).UTC()
	ms.AddAPIKey(store.APIKey{
		ID:        "key-future",
		Name:      "future",
		KeyPrefix: "sl_futur",
		KeyHash:   store.HashKey(futureToken),
		Scopes:    []string{store.ScopeReadContracts},
		CreatedAt: time.Now().UTC(),
		ExpiresAt: &future,
	})

	srv := newRequireAuthTestRouter(ms, false)

	// Expired key should be rejected with 401
	reqExpired := httptest.NewRequest(http.MethodGet, "/api/v1/stats/global", nil)
	reqExpired.Header.Set("Authorization", "Bearer "+expiredToken)
	wExpired := httptest.NewRecorder()
	srv.ServeHTTP(wExpired, reqExpired)
	if wExpired.Code != http.StatusUnauthorized {
		t.Errorf("expired key: want 401, got %d", wExpired.Code)
	}

	// Active key with future expiration should succeed
	reqFuture := httptest.NewRequest(http.MethodGet, "/api/v1/stats/global", nil)
	reqFuture.Header.Set("Authorization", "Bearer "+futureToken)
	wFuture := httptest.NewRecorder()
	srv.ServeHTTP(wFuture, reqFuture)
	if wFuture.Code != http.StatusOK {
		t.Errorf("unexpired key: want 200, got %d", wFuture.Code)
	}
}

func TestAdminAuditRoute(t *testing.T) {
	ms := store.NewMockStore()
	adminToken := "sl_admin_token_test"
	ms.AddAPIKey(store.APIKey{
		ID:        "key-admin",
		Name:      "admin",
		KeyPrefix: "sl_admin",
		KeyHash:   store.HashKey(adminToken),
		Scopes:    []string{store.ScopeAdmin},
		CreatedAt: time.Now().UTC(),
	})

	_ = ms.InsertAuditEvent(context.Background(), store.AuditEvent{
		Actor:  "admin",
		Action: "POST /admin/keys",
		Status: http.StatusCreated,
		At:     time.Now().UTC(),
	})

	srv := newRequireAuthTestRouter(ms, false)

	// Root /admin/audit without auth should be 401/403
	reqUnauth := httptest.NewRequest(http.MethodGet, "/admin/audit", nil)
	wUnauth := httptest.NewRecorder()
	srv.ServeHTTP(wUnauth, reqUnauth)
	if wUnauth.Code != http.StatusUnauthorized && wUnauth.Code != http.StatusForbidden {
		t.Errorf("unauthenticated GET /admin/audit: want 401 or 403, got %d", wUnauth.Code)
	}

	// Root /admin/audit with admin auth should return 200 and list events
	reqAuth := httptest.NewRequest(http.MethodGet, "/admin/audit", nil)
	reqAuth.Header.Set("Authorization", "Bearer "+adminToken)
	wAuth := httptest.NewRecorder()
	srv.ServeHTTP(wAuth, reqAuth)
	if wAuth.Code != http.StatusOK {
		t.Fatalf("GET /admin/audit with admin auth: want 200, got %d (body: %s)", wAuth.Code, wAuth.Body.String())
	}

	var resp struct {
		Events []struct {
			Action string `json:"action"`
		} `json:"events"`
	}
	if err := json.Unmarshal(wAuth.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal /admin/audit response: %v", err)
	}
	if len(resp.Events) == 0 {
		t.Errorf("expected audit events in response, got 0")
	}
}
