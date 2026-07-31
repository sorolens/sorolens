package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireJSONContentType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		method      string
		contentType string
		setHeader   bool
		wantStatus  int
		wantNextHit bool
	}{
		{
			name:        "POST without Content-Type is rejected",
			method:      http.MethodPost,
			setHeader:   false,
			wantStatus:  http.StatusUnsupportedMediaType,
			wantNextHit: false,
		},
		{
			name:        "POST with application/json passes through",
			method:      http.MethodPost,
			contentType: "application/json",
			setHeader:   true,
			wantStatus:  http.StatusOK,
			wantNextHit: true,
		},
		{
			name:        "POST with application/json and charset passes through",
			method:      http.MethodPost,
			contentType: "application/json; charset=utf-8",
			setHeader:   true,
			wantStatus:  http.StatusOK,
			wantNextHit: true,
		},
		{
			name:        "POST with wrong Content-Type is rejected",
			method:      http.MethodPost,
			contentType: "text/plain",
			setHeader:   true,
			wantStatus:  http.StatusUnsupportedMediaType,
			wantNextHit: false,
		},
		{
			name:        "PUT without Content-Type is rejected",
			method:      http.MethodPut,
			setHeader:   false,
			wantStatus:  http.StatusUnsupportedMediaType,
			wantNextHit: false,
		},
		{
			name:        "PUT with application/json passes through",
			method:      http.MethodPut,
			contentType: "application/json",
			setHeader:   true,
			wantStatus:  http.StatusOK,
			wantNextHit: true,
		},
		{
			name:        "GET without Content-Type is not affected",
			method:      http.MethodGet,
			setHeader:   false,
			wantStatus:  http.StatusOK,
			wantNextHit: true,
		},
		{
			name:        "GET with wrong Content-Type is not affected",
			method:      http.MethodGet,
			contentType: "text/plain",
			setHeader:   true,
			wantStatus:  http.StatusOK,
			wantNextHit: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			nextHit := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextHit = true
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(tc.method, "/", nil)
			if tc.setHeader {
				req.Header.Set("Content-Type", tc.contentType)
			}
			rec := httptest.NewRecorder()

			RequireJSONContentType(next).ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if nextHit != tc.wantNextHit {
				t.Errorf("next handler called = %v, want %v", nextHit, tc.wantNextHit)
			}

			if tc.wantStatus == http.StatusUnsupportedMediaType {
				if got := rec.Header().Get("Content-Type"); got != "application/json" {
					t.Errorf("response Content-Type = %q, want application/json", got)
				}
				if rec.Body.Len() == 0 {
					t.Error("expected a JSON error body, got empty response")
				}
			}
		})
	}
}
