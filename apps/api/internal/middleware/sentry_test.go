package middleware

import (
	"context"
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

// withTestSentry initializes a real Sentry client pointed at a fake
// transport, so the middleware's calls to sentry.CurrentHub() exercise the
// actual event-construction path (data-collection scrubbing included)
// without ever making a network call. sentry.Init is global state, so tests
// using it cannot run in parallel with each other.
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

func TestSentry_ReportsPanicAndRepanics(t *testing.T) {
	transport := withTestSentry(t)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/contracts/CABC123", nil)
	rec := httptest.NewRecorder()

	didPanic := func() (panicked bool) {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()
		Sentry(next).ServeHTTP(rec, req)
		return false
	}()

	if !didPanic {
		t.Fatal("expected Sentry middleware to re-panic so Recoverer still produces a 500")
	}

	events := transport.captured()
	if len(events) != 1 {
		t.Fatalf("events sent to Sentry: want 1, got %d", len(events))
	}
	if events[0].Level != sentry.LevelFatal && events[0].Level != sentry.LevelError {
		t.Errorf("event level = %v, want fatal or error", events[0].Level)
	}
}

func TestSentry_Reports5xxWithoutPanicking(t *testing.T) {
	transport := withTestSentry(t)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	req := httptest.NewRequest(http.MethodGet, "/contracts/CABC123", nil)
	rec := httptest.NewRecorder()

	Sentry(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}

	events := transport.captured()
	if len(events) != 1 {
		t.Fatalf("events sent to Sentry: want 1, got %d", len(events))
	}
	if got := events[0].Tags["status_code"]; got != "500" {
		t.Errorf("status_code tag = %q, want %q", got, "500")
	}
}

func TestSentry_DoesNotReportSuccessfulRequests(t *testing.T) {
	transport := withTestSentry(t)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/contracts/CABC123", nil)
	rec := httptest.NewRecorder()

	Sentry(next).ServeHTTP(rec, req)

	if events := transport.captured(); len(events) != 0 {
		t.Fatalf("events sent to Sentry: want 0, got %d", len(events))
	}
}

// TestSentry_ScrubsAuthorizationHeaderButKeepsContractID exercises
// sentry-go's own default data-collection deny-list (not custom scrubbing
// code in this package): it partial-matches header/query keys against terms
// like "auth" and "key", so the Authorization header and an api_key query
// parameter are redacted, while a contract ID, which only ever appears in
// the URL path, passes through untouched.
func TestSentry_ScrubsAuthorizationHeaderButKeepsContractID(t *testing.T) {
	transport := withTestSentry(t)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/contracts/CABC123?api_key=super-secret", nil)
	req.Header.Set("Authorization", "Bearer super-secret-token")
	rec := httptest.NewRecorder()

	func() {
		defer func() { recover() }() //nolint:errcheck
		Sentry(next).ServeHTTP(rec, req)
	}()

	events := transport.captured()
	if len(events) != 1 {
		t.Fatalf("events sent to Sentry: want 1, got %d", len(events))
	}
	event := events[0]
	if event.Request == nil {
		t.Fatal("event has no request data attached")
	}
	if got := event.Request.Headers["Authorization"]; got == "Bearer super-secret-token" {
		t.Error("Authorization header was not scrubbed")
	}
	if !strings.Contains(event.Request.QueryString, "Filtered") {
		t.Errorf("api_key query param was not scrubbed: %q", event.Request.QueryString)
	}
	if !strings.Contains(event.Request.URL, "CABC123") {
		t.Errorf("contract ID was unexpectedly scrubbed from the URL: %q", event.Request.URL)
	}
}
