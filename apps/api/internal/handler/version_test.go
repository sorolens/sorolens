package handler_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/router"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const versionPath = "/api/v1/version"

// newBuildInfoHandler returns a router whose handler reports the given build
// metadata, mirroring how main.go wires the ldflags-injected values.
func newBuildInfoHandler(info handler.BuildInfo) http.Handler {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	h := &handler.Handler{
		Store:       store.NewMockStore(),
		DB:          &store.MockPinger{Healthy: true},
		Redis:       &store.MockPinger{Healthy: true},
		RedisClient: &mockRedisClient{},
		Logger:      logger,
		BuildInfo:   info,
	}
	return router.New(h)
}

func TestVersionReturnsInjectedBuildInfo(t *testing.T) {
	srv := newBuildInfoHandler(handler.BuildInfo{
		Commit:    "d09cb51f1a2b3c4d",
		BuildDate: "2026-09-25T12:00:00Z",
	})

	// No credential is sent: the endpoint is public.
	req := httptest.NewRequest(http.MethodGet, versionPath, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("want Content-Type application/json, got %q", ct)
	}

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	for _, key := range []string{"commit", "build_date", "go_version", "api_version"} {
		if v, ok := body[key].(string); !ok || v == "" {
			t.Errorf("want non-empty string %q in response, got %v", key, body[key])
		}
	}
	if body["commit"] != "d09cb51f1a2b3c4d" {
		t.Errorf("want injected commit, got %v", body["commit"])
	}
	if body["build_date"] != "2026-09-25T12:00:00Z" {
		t.Errorf("want injected build_date, got %v", body["build_date"])
	}
	if body["api_version"] != handler.APIVersion {
		t.Errorf("want api_version=%q, got %v", handler.APIVersion, body["api_version"])
	}
	if v, _ := body["go_version"].(string); !strings.HasPrefix(v, "go") {
		t.Errorf("want a Go version in go_version, got %q", v)
	}
}

func TestVersionFallsBackToDefaults(t *testing.T) {
	// A binary built without ldflags must still answer with all four fields
	// rather than empty strings.
	srv := newBuildInfoHandler(handler.BuildInfo{})

	req := httptest.NewRequest(http.MethodGet, versionPath, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["commit"] != handler.DefaultCommit {
		t.Errorf("want commit=%q, got %q", handler.DefaultCommit, body["commit"])
	}
	if body["build_date"] != handler.DefaultBuildDate {
		t.Errorf("want build_date=%q, got %q", handler.DefaultBuildDate, body["build_date"])
	}
	if body["api_version"] != handler.APIVersion {
		t.Errorf("want api_version=%q, got %q", handler.APIVersion, body["api_version"])
	}
	if !strings.HasPrefix(body["go_version"], "go") {
		t.Errorf("want a Go version in go_version, got %q", body["go_version"])
	}
}
