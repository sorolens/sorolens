package handler_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/middleware"
	"github.com/sorolens/sorolens/apps/api/internal/router"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func newCachedServer(t *testing.T, ms *store.MockStore) http.Handler {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return router.New(&handler.Handler{
		Store:       ms,
		DB:          &store.MockPinger{Healthy: true},
		Redis:       &store.MockPinger{Healthy: true},
		RedisClient: &mockRedisClient{},
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Cache:       &middleware.RedisCache{Client: client},
		CacheTTL:    30 * time.Second,
	})
}

func TestResponseCacheOnContractRoutes(t *testing.T) {
	ms := seedRBACUsers(t)
	srv := newCachedServer(t, ms)

	xcache := func(path string) string {
		w := doRequest(srv, http.MethodGet, path, "", "")
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s: %d", path, w.Code)
		}
		return w.Header().Get("X-Cache")
	}

	if got := xcache("/api/v1/contracts"); got != "MISS" {
		t.Fatalf("first list: %q", got)
	}
	if got := xcache("/api/v1/contracts"); got != "HIT" {
		t.Fatalf("second list: %q", got)
	}
	if got := xcache("/api/v1/watchdog/stats"); got != "MISS" {
		t.Fatalf("first stats: %q", got)
	}

	// A write to contracts purges the contracts namespace only, and the
	// next read reflects it.
	if w := doRequestAsUser(srv, http.MethodPost, "/api/v1/contracts", "", contributorUser, validContractBody); w.Code != http.StatusCreated {
		t.Fatalf("register: %d %s", w.Code, w.Body.String())
	}
	w := doRequest(srv, http.MethodGet, "/api/v1/contracts", "", "")
	if w.Header().Get("X-Cache") != "MISS" || !strings.Contains(w.Body.String(), "CAAAAAAA") {
		t.Fatalf("read after write: X-Cache=%q body=%s", w.Header().Get("X-Cache"), w.Body.String())
	}
	if got := xcache("/api/v1/watchdog/stats"); got != "HIT" {
		t.Fatalf("watchdog stats must survive a contracts write: %q", got)
	}

	// A rejected write does not purge.
	xcache("/api/v1/contracts")
	doRequestAsUser(srv, http.MethodPost, "/api/v1/contracts", "", viewerUser, validContractBody)
	if got := xcache("/api/v1/contracts"); got != "HIT" {
		t.Fatalf("forbidden write purged the cache: %q", got)
	}

	// Hit/miss counters are exported on /metrics.
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	mw := httptest.NewRecorder()
	srv.ServeHTTP(mw, req)
	body := mw.Body.String()
	for _, want := range []string{
		`sorolens_api_cache_requests_total{namespace="contracts",result="hit"}`,
		`sorolens_api_cache_requests_total{namespace="contracts",result="miss"}`,
		`sorolens_api_cache_purges_total{namespace="contracts"}`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/metrics missing %s", want)
		}
	}
}

func TestResponseCacheDisabledWithoutRedis(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	w := doRequest(srv, http.MethodGet, "/api/v1/contracts", "", "")
	if w.Code != http.StatusOK || w.Header().Get("X-Cache") != "" {
		t.Fatalf("no cache configured: %d X-Cache=%q", w.Code, w.Header().Get("X-Cache"))
	}
}
