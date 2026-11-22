package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
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

func TestLogger_EnrichesRequestLog(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	r := chi.NewRouter()
	r.Use(RequestID)
	r.Use(Logger(logger))
	r.Get("/api/v1/contracts/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, "abc")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/contract-123", nil)
	req.RemoteAddr = "192.0.2.10:4321"
	req.Header.Set("User-Agent", "sorolens-test/1.0")
	req.Header.Set("X-Request-ID", "request-123")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	var record map[string]any
	if err := json.Unmarshal(logs.Bytes(), &record); err != nil {
		t.Fatalf("decode request log %q: %v", logs.String(), err)
	}
	want := map[string]any{
		"route_pattern":  "/api/v1/contracts/{id}",
		"client_ip":      "192.0.2.10",
		"user_agent":     "sorolens-test/1.0",
		"response_bytes": float64(3),
		"request_id":     "request-123",
	}
	for key, value := range want {
		if got := record[key]; got != value {
			t.Errorf("log field %q = %#v, want %#v", key, got, value)
		}
	}
	if _, ok := record["path"]; ok {
		t.Error("request log contains raw path instead of the chi route pattern")
	}
}
