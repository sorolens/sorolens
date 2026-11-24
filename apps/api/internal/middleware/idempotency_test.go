package middleware

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func idemRequest(h http.Handler, key string) (rec *httptest.ResponseRecorder, panicked bool) {
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"a":1}`))
	req.Header.Set(IdempotencyKeyHeader, key)
	req = req.WithContext(WithIdentity(req.Context(), VerifiedIdentity{
		Source: "test",
		APIKey: &store.APIKey{ID: "k1"},
	}))
	rec = httptest.NewRecorder()
	func() {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()
		h.ServeHTTP(rec, req)
	}()
	return rec, panicked
}

// A handler panic must not leave the key in-flight: a retry has to run the
// handler again instead of getting 409 until the key expires.
func TestIdempotencyReleasesKeyWhenHandlerPanics(t *testing.T) {
	ms := store.NewMockStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	calls := 0
	h := Idempotency(ms, logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			panic("boom")
		}
		w.WriteHeader(http.StatusCreated)
	}))

	if _, panicked := idemRequest(h, "k"); !panicked {
		t.Fatal("expected the panic to propagate to the recoverer")
	}
	if _, err := ms.GetIdempotencyRecord(context.Background(), "k1", "k"); err == nil {
		t.Fatal("in-flight key must be released after a panic")
	}

	rec, panicked := idemRequest(h, "k")
	if panicked || rec.Code != http.StatusCreated || calls != 2 {
		t.Fatalf("retry should re-run the handler: code=%d calls=%d panicked=%v", rec.Code, calls, panicked)
	}
	// And now it is completed and replayed.
	rec, _ = idemRequest(h, "k")
	if rec.Code != http.StatusCreated || rec.Header().Get("Idempotent-Replayed") != "true" || calls != 2 {
		t.Fatalf("expected replay, got code=%d calls=%d", rec.Code, calls)
	}
}
