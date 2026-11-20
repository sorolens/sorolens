package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

// uuidV4 matches the canonical textual form of a version 4 UUID: the version
// nibble is 4 and the variant nibble is one of 8, 9, a or b.
var uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// TestRequestIDGeneratesAndPropagates asserts a request without an ID gets a
// v4 UUID on the response and in the context the handler sees.
func TestRequestIDGeneratesAndPropagates(t *testing.T) {
	t.Parallel()

	var fromContext string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fromContext = GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil))

	header := rec.Header().Get("X-Request-ID")
	if header == "" {
		t.Fatal("X-Request-ID response header is empty")
	}
	if !uuidV4.MatchString(header) {
		t.Errorf("generated request ID %q is not a v4 UUID", header)
	}
	if fromContext != header {
		t.Errorf("context request ID = %q, want the response header %q", fromContext, header)
	}
}

// TestRequestIDPreservesCallerValue keeps a caller-supplied correlation ID so
// logs from an upstream proxy can be joined.
func TestRequestIDPreservesCallerValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		supply string
		want   string
	}{
		{"caller ID is kept verbatim", "trace-42", "trace-42"},
		{"caller ID is not trimmed or rewritten", "  spaced  ", "  spaced  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var fromContext string
			handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fromContext = GetRequestID(r.Context())
			}))

			req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil)
			req.Header.Set("X-Request-ID", tt.supply)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if got := rec.Header().Get("X-Request-ID"); got != tt.want {
				t.Errorf("response X-Request-ID = %q, want %q", got, tt.want)
			}
			if fromContext != tt.want {
				t.Errorf("context request ID = %q, want %q", fromContext, tt.want)
			}
		})
	}
}

// TestRequestIDIsUniquePerRequest guards against reusing one ID for every
// request, which would make traces useless.
func TestRequestIDIsUniquePerRequest(t *testing.T) {
	t.Parallel()

	ids := make(map[string]bool, 100)
	handler := RequestID(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	for i := 0; i < 100; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		id := rec.Header().Get("X-Request-ID")
		if ids[id] {
			t.Fatalf("duplicate request ID %q on request %d", id, i)
		}
		ids[id] = true
	}
}

// TestGetRequestIDWithoutMiddleware returns the zero value instead of
// panicking when no ID was attached.
func TestGetRequestIDWithoutMiddleware(t *testing.T) {
	t.Parallel()

	if got := GetRequestID(context.Background()); got != "" {
		t.Errorf("GetRequestID(background) = %q, want empty", got)
	}
	// A value stored under a different key must not be mistaken for the ID.
	ctx := context.WithValue(context.Background(), "X-Request-ID", "not-the-key")
	if got := GetRequestID(ctx); got != "" {
		t.Errorf("GetRequestID(unrelated key) = %q, want empty", got)
	}
}
