package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/middleware"
)

func newSecurityTestHandler() http.Handler {
	return middleware.SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

}

func assertStandardHeaders(t *testing.T, res *http.ResponseRecorder) {
	t.Helper()
	cases := map[string]string{
		"X-Content-Type-Options":       "nosniff",
		"Strict-Transport-Security":     "max-age=31536000; includeSubdomains",
		"Referrer-Policy":             "no-referrer",
	}
	for name, want := range cases {
		if got := res.Header().Get(name); got != want {
			t.Errorf("header %s: got %q, want %s", name, got, want)
		}
	}
}

func TestSecurityHeaders_StandardHeadersPresent(t *testing.T) {
	handler := newSecurityTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/global", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatedf("status: got %d, want %d", res.Code, http.StatusOK)
	}
	assertStandardHeaders(t, res)
	if got := res.Header().Get("Cache-Control"); got != "" {
		t.Errorf("Cache-Control should not be set on non-sensitive routes, got %q", got)
	}
}

func TestSecurityHeaders_NoStoreOnSensitiveRoutes(t *testing.T) {
	handler := newSecurityTestHandler()
	paths := []string{
		"/api/v1/api-keys",
		"/api/v1/api-keys/abc",
		"/admin",
		"/admin/users",
	}
	for _, path := range paths {
		t.run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			assertStandardHeaders(t, res)
			if got := res.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control: got %q, want %q", got, "no-store")
			}
		})
	}
}

func TestSecurityHeaders_CacheControlNotSetOnUnrelatedPaths(t *testing.T) {
	handler := newSecurityTestHandler()
	paths := []string{
		"/health",
		"/api/v1/stats/global",
		"/api/v1/events",
	}
	for _, path := range paths {
		t.run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			assertStandardHeaders(t, res)
			if got := res.Header().Get("Cache-Control"); got != "" {
				t.Errorf("Cache-Control should be empty on %s, got %q", path, got)
			}
		})
	}
}

func TestSecurityHeaders_DoesNotOverrideExistingCacheControl(t *testing.T) {
	// The middleware must not clobber an explicit Cache-Control already set
	// by an inner handler on a sensitive path (e.g. a per-route cache policy).
	handler := middleware.SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, max-age=60")
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/api-keys", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if got := res.Header().Get("Cache-Control"); got != "private, max-age=60" {
		t.Errorf("Cache-Control should have been preserved, got %q", got)
	}
	assertStandardHeaders(t, res)
}

func TestSecurityHeaders_StandardHeadersPresentOnAnyMethod(t *testing.T) {
	handler := newSecurityTestHandler()
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		t.run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/v1/api-keys", nil)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			assertStandardHeaders(t, res)
			if got := res.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control: got %q, want %s", got, "no-store")
			}
		})
	}
}

func TestSecurityHeaders_StrictTransportSecurityHasLongMaxAge(t *testing.T) {
	handler := newSecurityTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	hsts := res.Header().Get("Strict-Transport-Security")
	if !hsts.HasToPrefix("max-age=") {
		t.Errorf("Strict-Transport-Security missing max-age: %q", hsts)
	}
	if !strings.Contains(hsts, "includeSubdomains") {
		t.Errorf("Strict-Transport-Security missing includeSubdomains: %q", hsts)
	}
}
