package poller

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// ---- storage expiry alerting hook (issue #510) -----------------------------

// fakeExpiryAlerter records every end-of-pass invocation so the tests can pin
// the contract between the poller and the expiryalert job.
type fakeExpiryAlerter struct {
	mu    sync.Mutex
	calls int
	err   error
}

func (f *fakeExpiryAlerter) AlertExpiry(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.err
}

func (f *fakeExpiryAlerter) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// TestPoller_RunsExpiryAlerterOncePerPass is the scheduling half of issue #510:
// the scan must run after the pass, on the freshly written sync state, exactly
// once per pass.
func TestPoller_RunsExpiryAlerterOncePerPass(t *testing.T) {
	alerter := &fakeExpiryAlerter{}
	p := New(&fakeRPC{}, newFakeStore([]Contract{{ID: "CEXP", Status: "active"}}), newFakeRedis(), testConfig(), testLogger())
	p.SetExpiryAlerter(alerter)

	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := alerter.count(); got != 1 {
		t.Fatalf("expiry alerter calls = %d, want 1", got)
	}
}

// TestPoller_ExpiryAlerterErrorDoesNotFailPass keeps the job best-effort, like
// the anomaly and health-score passes: a storage scan that cannot run must never
// turn a successful indexing pass into a failed one.
func TestPoller_ExpiryAlerterErrorDoesNotFailPass(t *testing.T) {
	alerter := &fakeExpiryAlerter{err: errors.New("expiry scan unavailable")}
	p := New(&fakeRPC{}, newFakeStore(nil), newFakeRedis(), testConfig(), testLogger())
	p.SetExpiryAlerter(alerter)

	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("alerter error leaked into the pass: %v", err)
	}
	if got := alerter.count(); got != 1 {
		t.Fatalf("expiry alerter calls = %d, want 1", got)
	}
}

// TestPoller_NoExpiryAlerterIsNoOp proves deployments that never call
// SetExpiryAlerter behave exactly as before the feature landed.
func TestPoller_NoExpiryAlerterIsNoOp(t *testing.T) {
	p := New(&fakeRPC{}, newFakeStore(nil), newFakeRedis(), testConfig(), testLogger())
	if p.expiryAlerter != nil {
		t.Fatal("expiry alerting must be inert until it is attached")
	}
	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p.SetExpiryAlerter(nil) // explicit disable
	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
