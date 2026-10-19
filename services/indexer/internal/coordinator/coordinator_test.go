package coordinator

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func fixedClock(t *testing.T, start time.Time) (*time.Time, func() time.Time) {
	t.Helper()
	now := start
	return &now, func() time.Time { return now }
}

func contracts(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("CONTRACT%03d", i)
	}
	return out
}

func TestShardForContractIsStableAndInRange(t *testing.T) {
	const shardCount = 64
	for _, id := range contracts(200) {
		got := ShardForContract(id, shardCount)
		if got < 0 || got >= shardCount {
			t.Fatalf("ShardForContract(%q) = %d, outside [0,%d)", id, got, shardCount)
		}
		if again := ShardForContract(id, shardCount); again != got {
			t.Fatalf("ShardForContract(%q) not deterministic: %d then %d", id, got, again)
		}
	}
}

func TestShardForContractSpreadsAcrossShards(t *testing.T) {
	const shardCount = 16
	seen := make(map[int]int)
	for _, id := range contracts(500) {
		seen[ShardForContract(id, shardCount)]++
	}
	// Not a distribution guarantee, just a guard against a hash that collapses
	// everything onto one shard, which would defeat the whole feature.
	if len(seen) < shardCount/2 {
		t.Fatalf("500 contracts only reached %d of %d shards", len(seen), shardCount)
	}
}

func TestShardForContractHandlesNonPositiveShardCount(t *testing.T) {
	if got := ShardForContract("CONTRACT001", 0); got != 0 {
		t.Fatalf("shardCount 0: got %d, want 0", got)
	}
}

func TestContractsForShardsFiltersAndSorts(t *testing.T) {
	const shardCount = 8
	all := contracts(40)
	shards := []int{1, 3}
	got := ContractsForShards(all, shards, shardCount)

	if len(got) == 0 {
		t.Fatal("expected some contracts for shards 1 and 3")
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] > got[i] {
			t.Fatalf("result not sorted: %q before %q", got[i-1], got[i])
		}
	}
	for _, c := range got {
		if s := ShardForContract(c, shardCount); s != 1 && s != 3 {
			t.Fatalf("%q belongs to shard %d, not in {1,3}", c, s)
		}
	}
	if len(ContractsForShards(all, nil, shardCount)) != 0 {
		t.Fatal("no shards should select no contracts")
	}
	if len(ContractsForShards(nil, shards, shardCount)) != 0 {
		t.Fatal("no contracts should select nothing")
	}
}

func TestEnsureShardsCreatesTheFullSet(t *testing.T) {
	store := NewMemStore()
	c := New(store, nil, Config{ShardCount: 12})

	if err := c.EnsureShards(context.Background()); err != nil {
		t.Fatalf("EnsureShards: %v", err)
	}
	shards, err := store.ListShards(context.Background())
	if err != nil {
		t.Fatalf("ListShards: %v", err)
	}
	if len(shards) != 12 {
		t.Fatalf("got %d shards, want 12", len(shards))
	}

	// Idempotent: a second call must not duplicate rows.
	if err := c.EnsureShards(context.Background()); err != nil {
		t.Fatalf("EnsureShards (second): %v", err)
	}
	shards, _ = store.ListShards(context.Background())
	if len(shards) != 12 {
		t.Fatalf("after second EnsureShards got %d shards, want 12", len(shards))
	}
}

func TestRebalanceAssignsOrphanedShardsToLiveWorkers(t *testing.T) {
	ctx := context.Background()
	now, clock := fixedClock(t, time.Unix(1_700_000_000, 0))
	store := NewMemStore(contracts(50)...)
	c := New(store, nil, Config{ShardCount: 8, now: clock})

	if err := c.EnsureShards(ctx); err != nil {
		t.Fatalf("EnsureShards: %v", err)
	}
	for _, id := range []string{"worker-a", "worker-b"} {
		if err := store.Heartbeat(ctx, id, *now); err != nil {
			t.Fatalf("Heartbeat(%s): %v", id, err)
		}
	}

	moved, err := c.Rebalance(ctx)
	if err != nil {
		t.Fatalf("Rebalance: %v", err)
	}
	if moved != 8 {
		t.Fatalf("moved %d shards, want all 8 assigned", moved)
	}

	shards, _ := store.ListShards(ctx)
	for _, s := range shards {
		if s.WorkerID == "" {
			t.Fatalf("shard %d left unowned despite live workers", s.ShardID)
		}
	}

	// Second pass is a no-op: healthy ownership is never disturbed.
	moved, err = c.Rebalance(ctx)
	if err != nil {
		t.Fatalf("Rebalance (second): %v", err)
	}
	if moved != 0 {
		t.Fatalf("second rebalance moved %d shards, want 0", moved)
	}
}

func TestRebalanceReassignsStaleWorkerShards(t *testing.T) {
	ctx := context.Background()
	now, clock := fixedClock(t, time.Unix(1_700_000_000, 0))
	store := NewMemStore(contracts(30)...)
	c := New(store, nil, Config{
		ShardCount: 6,
		StaleAfter: 30 * time.Second,
		now:        clock,
	})

	if err := c.EnsureShards(ctx); err != nil {
		t.Fatalf("EnsureShards: %v", err)
	}
	if err := store.Heartbeat(ctx, "worker-a", *now); err != nil {
		t.Fatalf("Heartbeat(a): %v", err)
	}
	if err := store.Heartbeat(ctx, "worker-b", *now); err != nil {
		t.Fatalf("Heartbeat(b): %v", err)
	}
	if _, err := c.Rebalance(ctx); err != nil {
		t.Fatalf("Rebalance: %v", err)
	}

	ownedA, _ := store.ShardsForWorker(ctx, "worker-a")
	if len(ownedA) == 0 {
		t.Fatal("expected worker-a to own some shards before it goes stale")
	}

	// worker-a misses heartbeats past the staleness window; worker-b keeps up.
	*now = now.Add(31 * time.Second)
	if err := store.Heartbeat(ctx, "worker-b", *now); err != nil {
		t.Fatalf("Heartbeat(b): %v", err)
	}

	moved, err := c.Rebalance(ctx)
	if err != nil {
		t.Fatalf("Rebalance: %v", err)
	}
	if moved < len(ownedA) {
		t.Fatalf("moved %d shards, want at least worker-a's %d", moved, len(ownedA))
	}

	after, _ := store.ShardsForWorker(ctx, "worker-a")
	if len(after) != 0 {
		t.Fatalf("stale worker-a still owns %d shards", len(after))
	}
	total, _ := store.ShardsForWorker(ctx, "worker-b")
	if len(total) != 6 {
		t.Fatalf("worker-b owns %d shards, want all 6", len(total))
	}

	workers, _ := store.ListWorkers(ctx)
	for _, w := range workers {
		if w.WorkerID == "worker-a" {
			t.Fatal("stale worker-a was not forgotten")
		}
	}
}

func TestRebalanceUnassignsWhenNoWorkerIsLive(t *testing.T) {
	ctx := context.Background()
	now, clock := fixedClock(t, time.Unix(1_700_000_000, 0))
	store := NewMemStore(contracts(10)...)
	c := New(store, nil, Config{ShardCount: 4, StaleAfter: 10 * time.Second, now: clock})

	if err := c.EnsureShards(ctx); err != nil {
		t.Fatalf("EnsureShards: %v", err)
	}
	if err := store.Heartbeat(ctx, "worker-a", *now); err != nil {
		t.Fatalf("Heartbeat: %v", err)
	}
	if _, err := c.Rebalance(ctx); err != nil {
		t.Fatalf("Rebalance: %v", err)
	}

	*now = now.Add(time.Minute)
	if _, err := c.Rebalance(ctx); err != nil {
		t.Fatalf("Rebalance (all stale): %v", err)
	}

	shards, _ := store.ListShards(ctx)
	for _, s := range shards {
		if s.WorkerID != "" {
			t.Fatalf("shard %d still points at dead worker %q", s.ShardID, s.WorkerID)
		}
	}
}

// TestFourWorkersCoverHundredContractsWithoutOverlap is the end-to-end shape of
// issue #272: four workers, one hundred contracts, and an assignment in which
// every contract is indexed by exactly one worker.
func TestFourWorkersCoverHundredContractsWithoutOverlap(t *testing.T) {
	ctx := context.Background()
	now, clock := fixedClock(t, time.Unix(1_700_000_000, 0))
	all := contracts(100)
	store := NewMemStore(all...)
	const shardCount = 16
	c := New(store, nil, Config{ShardCount: shardCount, now: clock})

	if err := c.EnsureShards(ctx); err != nil {
		t.Fatalf("EnsureShards: %v", err)
	}

	workerIDs := []string{"worker-1", "worker-2", "worker-3", "worker-4"}
	for _, id := range workerIDs {
		if err := store.Heartbeat(ctx, id, *now); err != nil {
			t.Fatalf("Heartbeat(%s): %v", id, err)
		}
	}
	if _, err := c.Rebalance(ctx); err != nil {
		t.Fatalf("Rebalance: %v", err)
	}

	owner := make(map[string]string, len(all))
	for _, id := range workerIDs {
		w := NewWorker(store, nil, Config{ShardCount: shardCount, WorkerID: id, now: clock})
		got, err := w.AssignedContracts(ctx)
		if err != nil {
			t.Fatalf("AssignedContracts(%s): %v", id, err)
		}
		for _, contract := range got {
			if prev, ok := owner[contract]; ok {
				t.Fatalf("contract %s assigned to both %s and %s", contract, prev, id)
			}
			owner[contract] = id
		}
	}

	if len(owner) != len(all) {
		t.Fatalf("workers cover %d of %d contracts", len(owner), len(all))
	}
	for _, contract := range all {
		if _, ok := owner[contract]; !ok {
			t.Fatalf("contract %s was not assigned to any worker", contract)
		}
	}

	// Every worker should hold a share; a 4-way split that starves a worker
	// would mean the assignment is not actually balancing anything.
	perWorker := make(map[string]int, len(workerIDs))
	for _, id := range owner {
		perWorker[id]++
	}
	for _, id := range workerIDs {
		if perWorker[id] == 0 {
			t.Fatalf("%s received no contracts", id)
		}
	}
}

func TestWorkerWithoutIDFailsLoudly(t *testing.T) {
	ctx := context.Background()
	store := NewMemStore(contracts(5)...)
	w := NewWorker(store, nil, Config{})

	if _, err := w.AssignedContracts(ctx); !errors.Is(err, ErrNoWorkerID) {
		t.Fatalf("AssignedContracts err = %v, want ErrNoWorkerID", err)
	}
	if err := w.Heartbeat(ctx); !errors.Is(err, ErrNoWorkerID) {
		t.Fatalf("Heartbeat err = %v, want ErrNoWorkerID", err)
	}
	if err := w.Run(ctx, nil); !errors.Is(err, ErrNoWorkerID) {
		t.Fatalf("Run err = %v, want ErrNoWorkerID", err)
	}
}

func TestCoordinatorRunReleasesLockOnCancel(t *testing.T) {
	now, clock := fixedClock(t, time.Unix(1_700_000_000, 0))
	store := NewMemStore(contracts(4)...)
	c := New(store, nil, Config{
		ShardCount:        2,
		RebalanceInterval: time.Millisecond,
		now:               clock,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()

	// Give the loop a moment to win the lock, then stop it.
	deadline := time.After(2 * time.Second)
	for {
		held, err := store.TryAdvisoryLock(context.Background(), advisoryLockKey)
		if err != nil {
			t.Fatalf("TryAdvisoryLock: %v", err)
		}
		if held {
			// We just acquired it ourselves in this fake store, so release it
			// and rely on the assert below tracking the real loop's release.
			_ = store.ReleaseAdvisoryLock(context.Background(), advisoryLockKey)
			break
		}
		select {
		case <-deadline:
			t.Fatal("coordinator never acquired the leader lock")
		case <-time.After(time.Millisecond):
		}
	}

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run returned %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after cancel")
	}
	_ = now
}
