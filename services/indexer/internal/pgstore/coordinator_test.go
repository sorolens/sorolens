package pgstore

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sorolens/sorolens/services/indexer/internal/coordinator"
)

// testLockKey mirrors coordinator's unexported leader-lock key: the ASCII
// bytes of "Sorolens".
const testLockKey int64 = 0x536F726F6C656E73

// connectPair opens two independent pools so the two CoordinatorStores get
// distinct Postgres sessions, the way two replicas would.
func connectPair(t *testing.T) (*CoordinatorStore, *CoordinatorStore, *Store, *Store) {
	t.Helper()
	dsn := os.Getenv("POSTGRES_TEST_URL")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_URL not set, skipping DB test")
	}
	ctx := context.Background()
	a, err := Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect a: %v", err)
	}
	b, err := Connect(ctx, dsn)
	if err != nil {
		a.Close()
		t.Fatalf("connect b: %v", err)
	}
	t.Cleanup(func() { a.Close(); b.Close() })
	return NewCoordinatorStore(a.Pool()), NewCoordinatorStore(b.Pool()), a, b
}

func resetShardTables(t *testing.T, s *Store) {
	t.Helper()
	if _, err := s.Pool().Exec(context.Background(),
		`TRUNCATE indexer_shards, indexer_workers CASCADE`); err != nil {
		t.Fatalf("truncate shard tables: %v", err)
	}
}

// TestCoordinatorStore_Failover runs two coordinator replicas against one
// database. Only one can lead; when the leader goes away the survivor takes
// over and reassigns the dead worker's shards.
func TestCoordinatorStore_Failover(t *testing.T) {
	ca, cb, sa, _ := connectPair(t)
	t.Cleanup(func() { ca.Close(); cb.Close() })
	resetShardTables(t, sa)

	ctx := context.Background()
	prefix := uniquePrefix(t)
	ids := seedContracts(t, sa, prefix, "a", "b", "c")

	// Leadership is exclusive across sessions.
	ok, err := ca.TryAdvisoryLock(ctx, testLockKey)
	if err != nil || !ok {
		t.Fatalf("first leader: acquired=%v err=%v", ok, err)
	}
	ok, err = cb.TryAdvisoryLock(ctx, testLockKey)
	if err != nil {
		t.Fatalf("second replica: %v", err)
	}
	if ok {
		t.Fatal("two replicas acquired the leader lock at once")
	}

	// Replica A leads: four shards, all owned by worker-1. The worker's
	// heartbeat is an hour old, still inside A's two-hour staleness window.
	worker1, worker2 := "worker-1", "worker-2"
	if err := ca.Heartbeat(ctx, worker1, time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("heartbeat worker-1: %v", err)
	}
	cfgA := coordinator.Config{ShardCount: 4, WorkerID: "coordinator-a", StaleAfter: 2 * time.Hour}
	caCoord := coordinator.New(ca, nil, cfgA)
	if err := caCoord.EnsureShards(ctx); err != nil {
		t.Fatalf("ensure shards: %v", err)
	}
	moved, err := caCoord.Rebalance(ctx)
	if err != nil || moved != 4 {
		t.Fatalf("first rebalance: moved=%d err=%v, want 4", moved, err)
	}
	shards, err := ca.ListShards(ctx)
	if err != nil || len(shards) != 4 {
		t.Fatalf("shards = %d, %v; want 4", len(shards), err)
	}
	for _, sh := range shards {
		if sh.WorkerID != worker1 || sh.AssignedAt == nil {
			t.Fatalf("shard %d owned by %q (%v), want %s", sh.ShardID, sh.WorkerID, sh.AssignedAt, worker1)
		}
	}

	// The worker's shards resolve to its contract set.
	owned, err := ca.ShardsForWorker(ctx, worker1)
	if err != nil || len(owned) != 4 {
		t.Fatalf("shards for worker-1 = %v, %v; want 4", owned, err)
	}
	contracts, err := ca.ListContracts(ctx)
	if err != nil {
		t.Fatalf("list contracts: %v", err)
	}
	assigned := coordinator.ContractsForShards(contracts, owned, 4)
	got := make(map[string]bool, len(assigned))
	for _, id := range assigned {
		got[id] = true
	}
	for _, id := range ids {
		if !got[id] {
			t.Errorf("contract %s missing from worker-1's assignment (%v)", id, assigned)
		}
	}

	// Kill replica A by giving up the leader lock. Replica B can now lead.
	if err := ca.ReleaseAdvisoryLock(ctx, testLockKey); err != nil {
		t.Fatalf("release leader lock: %v", err)
	}
	ok, err = cb.TryAdvisoryLock(ctx, testLockKey)
	if err != nil || !ok {
		t.Fatalf("surviving replica: acquired=%v err=%v, want true", ok, err)
	}

	// Worker-1 has stopped heartbeating; worker-2 is alive. B's staleness
	// window is one minute, so worker-1 is dead and its shards must move.
	if err := cb.Heartbeat(ctx, worker2, time.Now()); err != nil {
		t.Fatalf("heartbeat worker-2: %v", err)
	}
	cfgB := coordinator.Config{ShardCount: 4, WorkerID: "coordinator-b", StaleAfter: time.Minute}
	cbCoord := coordinator.New(cb, nil, cfgB)
	moved, err = cbCoord.Rebalance(ctx)
	if err != nil || moved != 4 {
		t.Fatalf("failover rebalance: moved=%d err=%v, want 4", moved, err)
	}
	shards, err = cb.ListShards(ctx)
	if err != nil {
		t.Fatalf("list shards after failover: %v", err)
	}
	for _, sh := range shards {
		if sh.WorkerID != worker2 {
			t.Errorf("after failover shard %d owned by %q, want %s", sh.ShardID, sh.WorkerID, worker2)
		}
	}
	workers, err := cb.ListWorkers(ctx)
	if err != nil {
		t.Fatalf("list workers: %v", err)
	}
	if len(workers) != 1 || workers[0].WorkerID != worker2 {
		t.Errorf("workers after failover = %+v, want only %s", workers, worker2)
	}
}

// TestCoordinatorStore_ShardCRUD covers the table semantics the coordinator
// relies on: idempotent shard creation, owner updates, heartbeat monotonicity
// and the ON DELETE SET NULL cleanup when a worker is forgotten.
func TestCoordinatorStore_ShardCRUD(t *testing.T) {
	s, _, st, _ := connectPair(t)
	t.Cleanup(s.Close)
	resetShardTables(t, st)

	ctx := context.Background()

	for i := 0; i < 2; i++ {
		if err := s.EnsureShard(ctx, 7); err != nil {
			t.Fatalf("EnsureShard round %d: %v", i, err)
		}
	}
	shards, err := s.ListShards(ctx)
	if err != nil || len(shards) != 1 || shards[0].ShardID != 7 || shards[0].WorkerID != "" {
		t.Fatalf("after EnsureShard: %+v, %v", shards, err)
	}

	first := time.Now().Add(-time.Hour).UTC().Truncate(time.Microsecond)
	if err := s.Heartbeat(ctx, "w1", first); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	// An older heartbeat must not move liveness backwards, and started_at
	// keeps the first contact.
	if err := s.Heartbeat(ctx, "w1", first.Add(-30*time.Minute)); err != nil {
		t.Fatalf("stale heartbeat: %v", err)
	}
	workers, err := s.ListWorkers(ctx)
	if err != nil || len(workers) != 1 {
		t.Fatalf("workers = %+v, %v", workers, err)
	}
	if !workers[0].StartedAt.Equal(first) || !workers[0].HeartbeatAt.Equal(first) {
		t.Errorf("worker times = started %v / heartbeat %v, want both %v",
			workers[0].StartedAt, workers[0].HeartbeatAt, first)
	}

	assignedAt := time.Now().UTC().Truncate(time.Microsecond)
	if err := s.AssignShard(ctx, 7, "w1", assignedAt); err != nil {
		t.Fatalf("AssignShard: %v", err)
	}
	if err := s.AssignShard(ctx, 99, "w1", assignedAt); err == nil {
		t.Fatal("assigning a missing shard should fail")
	}
	shards, _ = s.ListShards(ctx)
	if len(shards) != 1 || shards[0].WorkerID != "w1" || shards[0].AssignedAt == nil {
		t.Fatalf("after assign: %+v", shards)
	}

	ids, err := s.ShardsForWorker(ctx, "w1")
	if err != nil || len(ids) != 1 || ids[0] != 7 {
		t.Fatalf("ShardsForWorker = %v, %v", ids, err)
	}

	// Deleting the worker nulls its shard ownership instead of deleting the
	// shard, so the next rebalance hands it out again.
	if err := s.DeleteWorker(ctx, "w1"); err != nil {
		t.Fatalf("DeleteWorker: %v", err)
	}
	shards, _ = s.ListShards(ctx)
	if len(shards) != 1 || shards[0].WorkerID != "" || shards[0].AssignedAt != nil {
		t.Fatalf("after DeleteWorker: %+v", shards)
	}

	if err := s.UnassignShard(ctx, 7); err != nil {
		t.Fatalf("UnassignShard: %v", err)
	}
	if err := s.UnassignShard(ctx, 99); err == nil {
		t.Fatal("unassigning a missing shard should fail")
	}
}
