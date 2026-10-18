package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

// failingWriter accepts limit bytes and then fails. It stands in for a client
// that disconnects mid-download.
type failingWriter struct {
	limit int
	n     int
}

var errWriteFailed = errors.New("write failed")

func (f *failingWriter) Write(p []byte) (int, error) {
	if f.n+len(p) > f.limit {
		return 0, errWriteFailed
	}
	f.n += len(p)
	return len(p), nil
}

// TestStreamEventsCSVReportsFlushOnlyFailures pins the flush-time error path:
// csv.Writer buffers, so a destination that fails on the very first write does
// not report anything from Write and only surfaces on Flush. Reporting success
// here would tell the handler a truncated download was delivered.
func TestStreamEventsCSVReportsFlushOnlyFailures(t *testing.T) {
	m := NewMockStore()
	if err := m.BatchInsertEvents(context.Background(), []Event{{
		ID:             "e1",
		ContractID:     "c1",
		Network:        "testnet",
		Ledger:         1,
		LedgerClosedAt: time.Unix(1700000000, 0).UTC(),
		Type:           "contract",
		ValueXDR:       `AAAA`,
	}}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// A zero limit fails on the first underlying write, which happens during
	// the final Flush rather than from csvWriter.Write.
	w := &failingWriter{limit: 0}
	err := m.StreamEventsCSV(context.Background(), "c1", EventFilters{}, w)
	if !errors.Is(err, errWriteFailed) {
		t.Fatalf("expected flush-time write failure to be reported, got %v", err)
	}
}

// TestStreamEventsCSVHeaderOnlySucceeds guards the other side of the boundary:
// a destination that accepts the header but not the row must still fail,
// while one that accepts everything must succeed.
func TestStreamEventsCSVPartialWriteFails(t *testing.T) {
	m := newCSVTestStore(t)

	w := &failingWriter{limit: 1}
	if err := m.StreamEventsCSV(context.Background(), "c1", EventFilters{}, w); err == nil {
		t.Fatal("expected failure when the destination rejects part of the body")
	}
}

// TestStreamEventsCSVFullyAcceptedSucceeds is the control case: no truncation
// means no error, so the flush fix cannot turn healthy exports into failures.
func TestStreamEventsCSVFullyAcceptedSucceeds(t *testing.T) {
	m := newCSVTestStore(t)

	var sb writeCounter
	if err := m.StreamEventsCSV(context.Background(), "c1", EventFilters{}, &sb); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if sb.n == 0 {
		t.Fatal("expected a non-empty CSV body")
	}
}

// newCSVTestStore returns a mock holding two ordered c1 events.
func newCSVTestStore(t *testing.T) *MockStore {
	t.Helper()
	m := NewMockStore()
	if err := m.BatchInsertEvents(context.Background(), []Event{
		{ID: "e1", ContractID: "c1", Network: "testnet", Ledger: 1, Type: "contract", ValueXDR: "AAAA"},
		{ID: "e2", ContractID: "c1", Network: "testnet", Ledger: 2, Type: "contract", ValueXDR: "BBBB"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return m
}

type writeCounter struct{ n int }

func (c *writeCounter) Write(p []byte) (int, error) {
	c.n += len(p)
	return len(p), nil
}
