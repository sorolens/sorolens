package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// panicEnvelope mirrors the error envelope Recoverer writes.
type panicEnvelope struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	} `json:"error"`
}

// TestRecovererReturnsInternalErrorEnvelope asserts a panic becomes a 500
// JSON envelope carrying the request ID rather than a dropped connection.
func TestRecovererReturnsInternalErrorEnvelope(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	h := RequestID(Recoverer(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("kaboom")
	})))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil)
	req.Header.Set("X-Request-ID", "req-500")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	var env panicEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode envelope %q: %v", rec.Body.String(), err)
	}
	if env.Error.Code != "INTERNAL" {
		t.Errorf("error.code = %q, want INTERNAL", env.Error.Code)
	}
	if env.Error.Message != "internal server error" {
		t.Errorf("error.message = %q, want %q", env.Error.Message, "internal server error")
	}
	if env.Error.RequestID != "req-500" {
		t.Errorf("error.request_id = %q, want req-500", env.Error.RequestID)
	}
}

// TestRecovererLogsThePanic asserts the stack and request ID are logged, since
// that log line is the only diagnostic for the 500 the caller sees.
func TestRecovererLogsThePanic(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	h := RequestID(Recoverer(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom with detail")
	})))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts", nil)
	req.Header.Set("X-Request-ID", "req-logged")
	h.ServeHTTP(httptest.NewRecorder(), req)

	var record map[string]any
	if err := json.Unmarshal(logs.Bytes(), &record); err != nil {
		t.Fatalf("decode log line %q: %v", logs.String(), err)
	}
	if record["msg"] != "panic recovered" {
		t.Errorf("msg = %v, want panic recovered", record["msg"])
	}
	if record["error"] != "boom with detail" {
		t.Errorf("error = %v, want the panic value", record["error"])
	}
	if record["request_id"] != "req-logged" {
		t.Errorf("request_id = %v, want req-logged", record["request_id"])
	}
	stack, _ := record["stack"].(string)
	if !strings.Contains(stack, "goroutine") {
		t.Errorf("stack = %q, want a debug.Stack() dump", stack)
	}
}

// TestRecovererPassesThroughNormalRequests covers the non-panic path: the
// handler's status and body must survive untouched and nothing is logged.
func TestRecovererPassesThroughNormalRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		status     int
		body       string
		wantLogged bool
	}{
		{"no panic, 200", http.StatusOK, "ok", false},
		{"no panic, handler error", http.StatusBadRequest, `{"error":"bad"}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, nil))
			h := Recoverer(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

			if rec.Code != tt.status {
				t.Errorf("status = %d, want %d", rec.Code, tt.status)
			}
			if rec.Body.String() != tt.body {
				t.Errorf("body = %q, want %q", rec.Body.String(), tt.body)
			}
			if tt.wantLogged != (logs.Len() > 0) {
				t.Errorf("logged = %v, want %v", logs.Len() > 0, tt.wantLogged)
			}
		})
	}
}

// TestRecovererToleratesNilLogger asserts panic handling does not itself
// panic when no logger is wired.
func TestRecovererToleratesNilLogger(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	Recoverer(nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("no logger")
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}
