package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// TestLogger_ForwardsFlush guards the SSE route: handlers behind Logger
// assert w.(http.Flusher), which the plain embedded writer did not satisfy.
func TestLogger_ForwardsFlush(t *testing.T) {
	var flushable bool
	h := Logger(discardLogger())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var f http.Flusher
		f, flushable = w.(http.Flusher)
		if flushable {
			_, _ = io.WriteString(w, "x")
			f.Flush()
		}
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	if !flushable {
		t.Fatal("writer behind Logger is not an http.Flusher")
	}
	if !rr.Flushed {
		t.Error("Flush was not forwarded to the underlying writer")
	}
}

// TestLogger_UnwrapsForResponseController checks http.ResponseController
// can reach the real writer through Logger.
func TestLogger_UnwrapsForResponseController(t *testing.T) {
	rr := httptest.NewRecorder()
	h := Logger(discardLogger())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok || u.Unwrap() != http.ResponseWriter(rr) {
			t.Errorf("Unwrap did not return the underlying writer")
		}
	}))
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
}
