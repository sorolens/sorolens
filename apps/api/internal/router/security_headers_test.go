package router_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func TestCORSOrigins(t *testing.T) {
	const dashboard = "https://dashboard.example"
	tests := []struct {
		name, config, origin, want string
	}{
		{"matching", "https://other.example, " + dashboard, dashboard, dashboard},
		{"denied", dashboard, dashboard + ".example", ""},
		{"no origin", dashboard, "", ""},
		{"unset", "", dashboard, "*"},
		{"wildcard", "*", dashboard, "*"},
		{"wildcard in list", dashboard + ",*", "https://other.example", "*"},
		{"empty allowlist entries", ", ,", dashboard, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CORS_ALLOWED_ORIGINS", tt.config)
			router := newMetricsTestRouter()
			for _, method := range []string{http.MethodGet, http.MethodOptions} {
				req := httptest.NewRequest(method, "/health", nil)
				req.Header.Set("Origin", tt.origin)
				req.Header.Set("Access-Control-Request-Method", "GET")
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tt.want {
					t.Errorf("%s: allow origin = %q, want %q", method, got, tt.want)
				}
				if !slices.Contains(rec.Header().Values("Vary"), "Origin") {
					t.Errorf("%s: missing Vary: Origin in %v", method, rec.Header())
				}
				if tt.want == "" && rec.Header().Get("Access-Control-Max-Age") != "" {
					t.Errorf("%s: denied origin received preflight cache age", method)
				}
			}
		})
	}
}

func TestSecurityHeadersAndPreflights(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://dashboard.example")
	router := newMetricsTestRouter()
	tests := []struct {
		path, requestedMethod string
		noStore               bool
	}{
		{"/health", "GET", false},
		{"/graphql", "POST", false},
		{"/api/v1/rules/123", "PATCH", false},
		{"/api/v2/contracts", "GET", false},
		{"/api/v1/api-keys/123", "DELETE", true},
		{"/api/v2/me/api-keys", "POST", true},
		{"/api/v2/admin/keys", "GET", true},
		{"/admin/audit", "GET", true},
		{"/api/v1/admin-tools", "GET", false},
	}
	wantMethods := []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	wantHeaders := []string{"Content-Type", "X-Request-ID", "Authorization", "X-API-Key", "If-None-Match", "X-Identity-User-ID", "X-Identity-Timestamp", "X-Identity-Signature"}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			for _, method := range []string{http.MethodOptions, http.MethodGet} {
				req := httptest.NewRequest(method, tt.path, nil)
				req.Header.Set("Origin", "https://dashboard.example")
				req.Header.Set("Access-Control-Request-Method", tt.requestedMethod)
				req.Header.Set("Access-Control-Request-Headers", strings.Join(wantHeaders, ", "))
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				h := rec.Result().Header
				for name, want := range map[string]string{
					"X-Content-Type-Options":    "nosniff",
					"Strict-Transport-Security": "max-age=31536000",
					"Referrer-Policy":           "strict-origin-when-cross-origin",
				} {
					if got := h.Get(name); got != want {
						t.Errorf("%s: %s = %q, want %q", method, name, got, want)
					}
				}
				if got := h.Get("Cache-Control") == "no-store"; got != tt.noStore {
					t.Errorf("%s: Cache-Control = %q, want no-store: %v", method, h.Get("Cache-Control"), tt.noStore)
				}
				if method != http.MethodOptions {
					continue
				}
				if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
					t.Fatalf("preflight returned %d with body %q", rec.Code, rec.Body)
				}
				if age := h.Get("Access-Control-Max-Age"); age != "600" {
					t.Errorf("preflight max age = %q", age)
				}
				if got := strings.Split(h.Get("Access-Control-Allow-Methods"), ", "); !slices.Equal(got, wantMethods) {
					t.Errorf("preflight methods = %v, want %v", got, wantMethods)
				}
				if got := strings.Split(h.Get("Access-Control-Allow-Headers"), ", "); !slices.Equal(got, wantHeaders) {
					t.Errorf("preflight headers = %v, want %v", got, wantHeaders)
				}
			}
		})
	}
}
