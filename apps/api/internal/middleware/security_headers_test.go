package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		path        string
		wantNoStore bool
	}{
		{name: "ordinary response", path: "/api/v1/contracts", wantNoStore: false},
		{name: "v1 api key collection", path: "/api/v1/api-keys", wantNoStore: true},
		{name: "v1 api key detail", path: "/api/v1/api-keys/key-1/rotate", wantNoStore: true},
		{name: "v1 admin response", path: "/api/v1/admin/audit", wantNoStore: true},
		{name: "v2 admin response", path: "/api/v2/admin/keys", wantNoStore: true},
		{name: "similar path is not sensitive", path: "/api/v1/administer", wantNoStore: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})).ServeHTTP(rec, req)

			for header, want := range map[string]string{
				"X-Content-Type-Options":    "nosniff",
				"Content-Security-Policy":   "default-src 'none'; frame-ancestors 'none'; base-uri 'none'",
				"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
				"Referrer-Policy":           "strict-origin-when-cross-origin",
			} {
				if got := rec.Header().Get(header); got != want {
					t.Errorf("%s = %q, want %q", header, got, want)
				}
			}

			if got := rec.Header().Get("Cache-Control"); tc.wantNoStore && got != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", got)
			} else if !tc.wantNoStore && got != "" {
				t.Errorf("Cache-Control = %q, want empty", got)
			}
		})
	}
}
