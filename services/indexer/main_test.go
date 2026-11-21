package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
)

// fakeTransport captures events instead of sending them to Sentry, so tests
// can assert on what would have been reported without any network access.
type fakeTransport struct {
	mu     sync.Mutex
	events []*sentry.Event
}

func (t *fakeTransport) Configure(sentry.ClientOptions) {}
func (t *fakeTransport) SendEvent(event *sentry.Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, event)
}
func (t *fakeTransport) Flush(time.Duration) bool              { return true }
func (t *fakeTransport) FlushWithContext(context.Context) bool { return true }
func (t *fakeTransport) Close()                                {}

func (t *fakeTransport) captured() []*sentry.Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]*sentry.Event, len(t.events))
	copy(out, t.events)
	return out
}

func withTestSentry(t *testing.T) *fakeTransport {
	t.Helper()
	transport := &fakeTransport{}
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:       "https://public@example.com/1",
		Transport: transport,
	}); err != nil {
		t.Fatalf("sentry.Init: %v", err)
	}
	t.Cleanup(func() {
		sentry.CurrentHub().BindClient(nil)
	})
	return transport
}

// TestReportPanic_ReportsThenRepanics simulates an unhandled panic in the
// indexer (issue #174's "Panic in a handler produces a Sentry event", read
// for this package as "in the indexing loop" since there is no HTTP
// handler): reportPanic must send an event to Sentry and then let the panic
// continue, so the process still crashes with a non-zero exit exactly as it
// did before this existed.
func TestReportPanic_ReportsThenRepanics(t *testing.T) {
	transport := withTestSentry(t)

	didPanic := func() (panicked bool) {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()
		defer reportPanic()
		panic("boom")
	}()

	if !didPanic {
		t.Fatal("expected reportPanic to re-panic after reporting")
	}

	events := transport.captured()
	if len(events) != 1 {
		t.Fatalf("events sent to Sentry: want 1, got %d", len(events))
	}
}

// TestReportPanic_NoOpWithoutSentry confirms issue #174's "Disabled cleanly
// when DSN is empty": with no Sentry client bound, reportPanic must still
// re-panic without erroring or blocking.
func TestReportPanic_NoOpWithoutSentry(t *testing.T) {
	sentry.CurrentHub().BindClient(nil)

	didPanic := func() (panicked bool) {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()
		defer reportPanic()
		panic("boom")
	}()

	if !didPanic {
		t.Fatal("expected reportPanic to re-panic even with no Sentry client bound")
	}
}

func TestEnvString(t *testing.T) {
	t.Setenv("SOROLENS_TEST_ENV_STRING", "")
	if got := envString("SOROLENS_TEST_ENV_STRING", "default"); got != "default" {
		t.Errorf("unset: got %q, want %q", got, "default")
	}

	t.Setenv("SOROLENS_TEST_ENV_STRING", "configured")
	if got := envString("SOROLENS_TEST_ENV_STRING", "default"); got != "configured" {
		t.Errorf("set: got %q, want %q", got, "configured")
	}
}

// fakePinger is a health dependency whose Ping result the test controls.
type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

// TestServeHealthz_FailingDependency checks that a failing dependency yields a
// 503 with an application/json Content-Type and a body that is valid JSON even
// when the dependency's error message contains quotes, backslashes and
// newlines. The old handler concatenated err.Error() into a string literal,
// which produced invalid JSON for exactly those characters.
func TestServeHealthz_FailingDependency(t *testing.T) {
	nasty := errors.New("boom \"quoted\" \\ backslash\nnewline")
	hc := &healthChecker{
		redis:    fakePinger{err: nasty},
		postgres: fakePinger{}, // healthy
	}

	rec := httptest.NewRecorder()
	hc.serveHealthz(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", got, "application/json")
	}

	var body struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid JSON: %v (body=%q)", err, rec.Body.String())
	}
	if body.Status != "fail" {
		t.Errorf("status field: got %q, want %q", body.Status, "fail")
	}
	if !strings.Contains(body.Error, "boom") {
		t.Errorf("error field: got %q, want it to include the dependency error", body.Error)
	}
}

// TestServeHealthz_Healthy checks the success path: 200, application/json and
// a valid JSON body that omits the error field.
func TestServeHealthz_Healthy(t *testing.T) {
	hc := &healthChecker{
		redis:    fakePinger{},
		postgres: fakePinger{},
	}

	rec := httptest.NewRecorder()
	hc.serveHealthz(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", got, "application/json")
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid JSON: %v (body=%q)", err, rec.Body.String())
	}
	if body["status"] != "ok" {
		t.Errorf("status field: got %v, want %q", body["status"], "ok")
	}
	if _, ok := body["error"]; ok {
		t.Errorf("healthy body must omit the error field, got %q", rec.Body.String())
	}
}
