package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCORSHeadersAndPassthrough covers the header contract on ordinary
// requests and the preflight short-circuit, which must not reach the handler.
func TestCORSHeadersAndPassthrough(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		wantStatus int
		wantNext   bool
	}{
		{"GET is passed to the handler", http.MethodGet, http.StatusOK, true},
		{"POST is passed to the handler", http.MethodPost, http.StatusOK, true},
		{"DELETE is passed to the handler", http.MethodDelete, http.StatusOK, true},
		{"OPTIONS preflight is answered directly", http.MethodOptions, http.StatusNoContent, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})

			rec := httptest.NewRecorder()
			CORS("")(next).ServeHTTP(rec, httptest.NewRequest(tt.method, "/api/v1/contracts", nil))

			if called != tt.wantNext {
				t.Errorf("next called = %v, want %v", called, tt.wantNext)
			}
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

// TestCORSPreflightHeaders asserts the preflight advertises the cross-origin
// contract the browser needs.
func TestCORSPreflightHeaders(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	CORS("")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/api/v1/events", nil))

	want := map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Headers": "Content-Type, X-Request-ID, Authorization, X-API-Key, If-None-Match, X-Identity-User-ID, X-Identity-Timestamp, X-Identity-Signature",
		"Access-Control-Allow-Methods": "GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS",
		"Access-Control-Max-Age":       "600",
	}
	for header, value := range want {
		if got := rec.Header().Get(header); got != value {
			t.Errorf("%s = %q, want %q", header, got, value)
		}
	}
}

// TestCORSKeepsHandlerStatusAndBody guards against the middleware replacing a
// response the handler already produced.
func TestCORSKeepsHandlerStatusAndBody(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	CORS("")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid API key"}`))
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/api-keys", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if body := rec.Body.String(); body != `{"error":"invalid API key"}` {
		t.Errorf("body = %q, want the handler body", body)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want *", got)
	}
}
