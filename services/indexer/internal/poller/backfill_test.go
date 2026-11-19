package poller

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// ---- fake store backfill surface -------------------------------------------

func (f *fakeStore) BeginBackfill(_ context.Context, p BackfillProgress) (BackfillProgress, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.backfills == nil {
		f.backfills = make(map[string]BackfillProgress)
	}
	if saved, ok := f.backfills[p.ContractID]; ok {
		p = saved
	} else {
		f.backfills[p.ContractID] = p
	}
	f.setStatusLocked(p.ContractID, "backfilling", "pending", "active")
	if f.syncStates[p.ContractID].LastLedger == 0 {
		f.syncStates[p.ContractID] = SyncState{ContractID: p.ContractID, LastLedger: p.NewestLedger}
	}
	return p, nil
}

func (f *fakeStore) CommitBackfillWindow(_ context.Context, w BackfillWindow) ([]FailedEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.insertErr != nil {
		return nil, f.insertErr
	}
	f.events = append(f.events, w.Events...)
	f.invocations = append(f.invocations, w.Invocations...)
	f.backfills[w.Progress.ContractID] = w.Progress
	if w.Complete {
		f.setStatusLocked(w.Progress.ContractID, "active", "backfilling")
	}
	return nil, nil
}

// setStatusLocked moves a contract to status when it is in one of from.
func (f *fakeStore) setStatusLocked(contractID, status string, from ...string) {
	for i := range f.contracts {
		if f.contracts[i].ID != contractID {
			continue
		}
		for _, s := range from {
			if f.contracts[i].Status == s {
				f.contracts[i].Status = status
				return
			}
		}
	}
}

func (f *fakeStore) status(contractID string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.contracts {
		if c.ID == contractID {
			return c.Status
		}
	}
	return ""
}

// windowRPC fails getEvents once a call's upper bound drops to failBelow, and
// reports a retention floor when floorErr is set.
type windowRPC struct {
	*fakeRPC
	failBelow uint32
	floorErr  error
}

func (r *windowRPC) GetEvents(ctx context.Context, start, end uint32, filters []EventFilter) (*GetEventsResult, error) {
	if r.floorErr != nil && start < 9_000 {
		return nil, r.floorErr
	}
	if r.failBelow > 0 && end <= r.failBelow {
		return nil, errors.New("rpc went away")
	}
	return r.fakeRPC.GetEvents(ctx, start, end, filters)
}

type floorError struct{ oldest uint32 }

func (e floorError) Error() string { return fmt.Sprintf("ledger range: %d - 10000", e.oldest) }

func (e floorError) OldestRetainedLedger() (uint32, bool) { return e.oldest, true }

func backfillConfig(window uint32) Config {
	cfg := testConfig()
	cfg.LedgerWindow = 100_000
	cfg.BackfillWindow = window
	return cfg
}

// ---- tests -----------------------------------------------------------------

func TestBackfill_WalksNewestToOldestInBoundedWindowsThenActivates(t *testing.T) {
	t.Parallel()

	id := "CBACKFILL"
	store := newFakeStore([]Contract{{ID: id, Status: "pending"}})
	rpc := &fakeRPC{latestLedger: &LatestLedger{Sequence: 2_500}}

	p := New(rpc, store, newFakeRedis(), backfillConfig(1_000), testLogger())
	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("run: %v", err)
	}

	want := []getEventsCall{
		{StartLedger: 1_501, EndLedger: 2_501},
		{StartLedger: 501, EndLedger: 1_501},
		{StartLedger: 1, EndLedger: 501},
	}
	if len(rpc.eventsCalls) != len(want) {
		t.Fatalf("getEvents calls: want %d, got %+v", len(want), rpc.eventsCalls)
	}
	for i, w := range want {
		got := rpc.eventsCalls[i]
		if got.StartLedger != w.StartLedger || got.EndLedger != w.EndLedger {
			t.Errorf("call %d: want [%d,%d), got [%d,%d)", i, w.StartLedger, w.EndLedger, got.StartLedger, got.EndLedger)
		}
	}
	if got := store.status(id); got != "active" {
		t.Errorf("status: want active, got %q", got)
	}
	if !store.backfills[id].Done() {
		t.Errorf("progress should be done: %+v", store.backfills[id])
	}
	if got := store.syncStates[id].LastLedger; got != 2_500 {
		t.Errorf("live cursor: want the backfill head 2500, got %d", got)
	}
}

func TestBackfill_ResumesFromSavedPositionAfterRestart(t *testing.T) {
	t.Parallel()

	id := "CRESUME"
	store := newFakeStore([]Contract{{ID: id, Status: "pending"}})
	base := &fakeRPC{latestLedger: &LatestLedger{Sequence: 3_000}}

	// The first process dies once the window ending at ledger 1000 is reached.
	first := New(&windowRPC{fakeRPC: base, failBelow: 1_001}, store, newFakeRedis(), backfillConfig(1_000), testLogger())
	if err := first.Run(context.Background(), "once"); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if got := store.status(id); got != "backfilling" {
		t.Fatalf("status after interruption: want backfilling, got %q", got)
	}
	saved := store.backfills[id]
	if saved.NextLedger != 1_000 {
		t.Fatalf("saved position: want next ledger 1000, got %d", saved.NextLedger)
	}

	// A new process, sharing only the store, picks up at the saved ledger even
	// though the chain head has moved on.
	resumedRPC := &fakeRPC{latestLedger: &LatestLedger{Sequence: 3_500}}
	second := New(resumedRPC, store, newFakeRedis(), backfillConfig(1_000), testLogger())
	if err := second.Run(context.Background(), "once"); err != nil {
		t.Fatalf("second run: %v", err)
	}

	var backward []getEventsCall
	for _, c := range resumedRPC.eventsCalls {
		if c.StartLedger <= saved.NextLedger {
			backward = append(backward, c)
		}
	}
	if len(backward) != 1 || backward[0].StartLedger != 1 || backward[0].EndLedger != 1_001 {
		t.Fatalf("resume: want one window [1,1001), got %+v", backward)
	}
	if got := store.status(id); got != "active" {
		t.Errorf("status: want active, got %q", got)
	}
	if got := store.backfills[id].NewestLedger; got != 3_000 {
		t.Errorf("the original range must survive a restart: newest %d", got)
	}
}

func TestBackfill_LiveEventsAreIndexedWhileBackfilling(t *testing.T) {
	t.Parallel()

	id := "CLIVE"
	store := newFakeStore([]Contract{{ID: id, Status: "backfilling"}})
	store.backfills = map[string]BackfillProgress{
		id: {ContractID: id, OldestLedger: 1, NewestLedger: 3_000, NextLedger: 2_000},
	}
	store.syncStates[id] = SyncState{ContractID: id, LastLedger: 3_000}
	rpc := &fakeRPC{latestLedger: &LatestLedger{Sequence: 3_100}}

	p := New(rpc, store, newFakeRedis(), backfillConfig(1_000), testLogger())
	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("run: %v", err)
	}
	if rpc.eventsCalls[0].StartLedger != 3_001 {
		t.Errorf("first call should tail the head from 3001, got %d", rpc.eventsCalls[0].StartLedger)
	}
	if got := store.syncStates[id].LastLedger; got != 3_100 {
		t.Errorf("live cursor: want 3100, got %d", got)
	}
}

func TestBackfill_StopsBetweenWindowsWhenCancelled(t *testing.T) {
	t.Parallel()

	id := "CSTOP"
	store := newFakeStore([]Contract{{ID: id, Status: "pending"}})
	rpc := &fakeRPC{latestLedger: &LatestLedger{Sequence: 5_000}}
	p := New(rpc, store, newFakeRedis(), backfillConfig(1_000), testLogger())

	stop, cancel := context.WithCancel(context.Background())
	rpc.onGetEvents = cancel // Shutdown arrives during the first window.

	if err := p.indexContract(context.Background(), stop, store.contracts[0]); err != nil {
		t.Fatalf("indexContract: %v", err)
	}
	if len(rpc.eventsCalls) != 1 {
		t.Fatalf("want only the in-flight window, got %d calls", len(rpc.eventsCalls))
	}
	if got := store.backfills[id].NextLedger; got != 4_000 {
		t.Errorf("in-flight window must commit: next ledger %d", got)
	}
	if got := store.status(id); got != "backfilling" {
		t.Errorf("status: want backfilling, got %q", got)
	}
}

func TestBackfill_ClampsToTheRetentionFloorAndCompletes(t *testing.T) {
	t.Parallel()

	id := "CFLOOR"
	store := newFakeStore([]Contract{{ID: id, Status: "pending"}})
	base := &fakeRPC{latestLedger: &LatestLedger{Sequence: 10_000}}
	rpc := &windowRPC{fakeRPC: base, floorErr: floorError{oldest: 9_500}}

	p := New(rpc, store, newFakeRedis(), backfillConfig(1_000), testLogger())
	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := store.status(id); got != "active" {
		t.Fatalf("status: want active, got %q", got)
	}
	if got := store.backfills[id].OldestLedger; got != 9_500 {
		t.Errorf("oldest ledger: want the retention floor 9500, got %d", got)
	}
}

func TestBackfill_FailedCommitKeepsThePosition(t *testing.T) {
	t.Parallel()

	id := "CCOMMIT"
	store := newFakeStore([]Contract{{ID: id, Status: "backfilling"}})
	store.backfills = map[string]BackfillProgress{
		id: {ContractID: id, OldestLedger: 1, NewestLedger: 2_000, NextLedger: 2_000},
	}
	store.syncStates[id] = SyncState{ContractID: id, LastLedger: 2_000}
	store.insertErr = errors.New("database unavailable")
	rpc := &fakeRPC{latestLedger: &LatestLedger{Sequence: 2_000}}

	p := New(rpc, store, newFakeRedis(), backfillConfig(1_000), testLogger())
	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := store.backfills[id].NextLedger; got != 2_000 {
		t.Errorf("position must not advance past an uncommitted window: %d", got)
	}
}

func TestBackfill_FinishesARangeThatIsAlreadyExhausted(t *testing.T) {
	t.Parallel()

	id := "CDONE"
	store := newFakeStore([]Contract{{ID: id, Status: "backfilling"}})
	store.backfills = map[string]BackfillProgress{
		id: {ContractID: id, OldestLedger: 100, NewestLedger: 2_000, NextLedger: 99},
	}
	store.syncStates[id] = SyncState{ContractID: id, LastLedger: 2_000}
	rpc := &fakeRPC{latestLedger: &LatestLedger{Sequence: 2_000}}

	p := New(rpc, store, newFakeRedis(), backfillConfig(1_000), testLogger())
	if err := p.Run(context.Background(), "once"); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(rpc.eventsCalls) != 0 {
		t.Errorf("no ledgers remain, got %d getEvents calls", len(rpc.eventsCalls))
	}
	if got := store.status(id); got != "active" {
		t.Errorf("status: want active, got %q", got)
	}
}

func TestBackfill_WindowNeverExceedsTheLedgerWindow(t *testing.T) {
	t.Parallel()

	p := New(&fakeRPC{}, newFakeStore(nil), newFakeRedis(), Config{LedgerWindow: 500, BackfillWindow: 1_000}, testLogger())
	if got := p.backfillWindowSize(); got != 500 {
		t.Errorf("window: want 500, got %d", got)
	}
	p = New(&fakeRPC{}, newFakeStore(nil), newFakeRedis(), Config{}, testLogger())
	if got := p.backfillWindowSize(); got != DefaultBackfillWindow {
		t.Errorf("default window: want %d, got %d", DefaultBackfillWindow, got)
	}
}
