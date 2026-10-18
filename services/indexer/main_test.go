package main

import (
	"context"
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
