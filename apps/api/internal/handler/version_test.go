package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/buildinfo"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// setBuildInfo temporarily overrides the build-time ldflags values so the
// handler can be exercised without a real -ldflags build. It returns a restore
// func to keep the package vars untouched for other tests.
func setBuildInfo(version, sha, builtAt string) func() {
	prevVersion, prevSHA, prevBuiltAt := buildinfo.Version, buildinfo.GitSHA, buildinfo.BuiltAt
	buildinfo.Version, buildinfo.GitSHA, buildinfo.BuiltAt = version, sha, builtAt
	return func() {
		buildinfo.Version, buildinfo.GitSHA, buildinfo.BuiltAt = prevVersion, prevSHA, prevBuiltAt
	}
}

func TestVersionDefaults(t *testing.T) {
	defer setBuildInfo("", "", "")()

	srv := newTestHandler(store.NewMockStore(), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("want Content-Type application/json, got %q", ct)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	for _, key := range []string{"version", "git_sha", "built_at"} {
		if body[key] != "dev" {
			t.Errorf("want %s=dev when unset, got %q", key, body[key])
		}
	}
}

func TestVersionInjected(t *testing.T) {
	defer setBuildInfo("1.4.2", "abc1234", "2026-01-02T15:04:05Z")()

	srv := newTestHandler(store.NewMockStore(), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["version"] != "1.4.2" {
		t.Errorf("want version=1.4.2, got %q", body["version"])
	}
	if body["git_sha"] != "abc1234" {
		t.Errorf("want git_sha=abc1234, got %q", body["git_sha"])
	}
	if body["built_at"] != "2026-01-02T15:04:05Z" {
		t.Errorf("want built_at=2026-01-02T15:04:05Z, got %q", body["built_at"])
	}
}

func TestVersionMalformedBuiltAtFallsBackToDev(t *testing.T) {
	defer setBuildInfo("1.4.2", "abc1234", "not-rfc3339")()

	srv := newTestHandler(store.NewMockStore(), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["built_at"] != "dev" {
		t.Errorf("want built_at=dev for malformed value, got %q", body["built_at"])
	}
}
