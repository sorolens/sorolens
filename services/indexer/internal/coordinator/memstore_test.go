package coordinator

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestMemStore_Contracts(t *testing.T) {
	ctx := context.Background()

	m := NewMemStore("C003", "C001", "C002")

	// ListContracts should return a sorted copy
	list, err := m.ListContracts(ctx)
	if err != nil {
		t.Fatalf("ListContracts error: %v", err)
	}
	expected := []string{"C001", "C002", "C003"}
	if !reflect.DeepEqual(list, expected) {
		t.Errorf("ListContracts = %v, want %v", list, expected)
	}

	// SetContracts replaces the list
	m.SetContracts([]string{"C010", "C005"})
	list, err = m.ListContracts(ctx)
	if err != nil {
		t.Fatalf("ListContracts error: %v", err)
	}
	expected = []string{"C005", "C010"}
	if !reflect.DeepEqual(list, expected) {
		t.Errorf("ListContracts after SetContracts = %v, want %v", list, expected)
	}
}

func TestMemStore_AdvisoryLock(t *testing.T) {
	ctx := context.Background()
	m := NewMemStore()

	// Initial acquisition succeeds
	acquired, err := m.TryAdvisoryLock(ctx, 42)
	if err != nil {
		t.Fatalf("TryAdvisoryLock error: %v", err)
	}
	if !acquired {
		t.Fatal("expected to acquire lock with key 42")
	}

	// Same key can re-acquire (idempotent for owner)
	reAcquired, err := m.TryAdvisoryLock(ctx, 42)
	if err != nil || !reAcquired {
		t.Fatalf("expected re-acquisition with same key to succeed, got (%v, %v)", reAcquired, err)
	}

	// Different key fails to acquire
	otherAcquired, err := m.TryAdvisoryLock(ctx, 99)
	if err != nil {
		t.Fatalf("TryAdvisoryLock error: %v", err)
	}
	if otherAcquired {
		t.Fatal("expected acquisition with different key 99 to fail")
	}

	// Releasing with wrong key does not release
	if err := m.ReleaseAdvisoryLock(ctx, 99); err != nil {
		t.Fatalf("ReleaseAdvisoryLock error: %v", err)
	}
	stillLocked, _ := m.TryAdvisoryLock(ctx, 99)
	if stillLocked {
		t.Fatal("lock should still be held by key 42")
	}

	// Releasing with correct key releases
	if err := m.ReleaseAdvisoryLock(ctx, 42); err != nil {
		t.Fatalf("ReleaseAdvisoryLock error: %v", err)
	}

	// Now key 99 can acquire
	acquiredNew, err := m.TryAdvisoryLock(ctx, 99)
	if err != nil || !acquiredNew {
		t.Fatalf("expected key 99 to acquire after release, got (%v, %v)", acquiredNew, err)
	}
}

func TestMemStore_Shards(t *testing.T) {
	ctx := context.Background()
	m := NewMemStore()
	now := time.Now().UTC()

	// EnsureShard creates shards
	if err := m.EnsureShard(ctx, 2); err != nil {
		t.Fatal(err)
	}
	if err := m.EnsureShard(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if err := m.EnsureShard(ctx, 1); err != nil {
		t.Fatal(err)
	}
	// Calling EnsureShard again is a no-op
	if err := m.EnsureShard(ctx, 1); err != nil {
		t.Fatal(err)
	}

	// ListShards returns sorted by ShardID
	shards, err := m.ListShards(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(shards) != 3 {
		t.Fatalf("expected 3 shards, got %d", len(shards))
	}
	if shards[0].ShardID != 0 || shards[1].ShardID != 1 || shards[2].ShardID != 2 {
		t.Errorf("shards not sorted: %v", shards)
	}

	// AssignShard on non-existent shard returns error
	if err := m.AssignShard(ctx, 99, "w1", now); err == nil {
		t.Fatal("expected error assigning non-existent shard")
	}

	// AssignShard on existing shard
	if err := m.AssignShard(ctx, 1, "w1", now); err != nil {
		t.Fatal(err)
	}
	if err := m.AssignShard(ctx, 2, "w1", now); err != nil {
		t.Fatal(err)
	}
	if err := m.AssignShard(ctx, 0, "w2", now); err != nil {
		t.Fatal(err)
	}

	// ShardsForWorker
	w1Shards, err := m.ShardsForWorker(ctx, "w1")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(w1Shards, []int{1, 2}) {
		t.Errorf("ShardsForWorker(w1) = %v, want [1, 2]", w1Shards)
	}

	w2Shards, err := m.ShardsForWorker(ctx, "w2")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(w2Shards, []int{0}) {
		t.Errorf("ShardsForWorker(w2) = %v, want [0]", w2Shards)
	}

	// Empty workerID returns nil
	emptyShards, err := m.ShardsForWorker(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(emptyShards) != 0 {
		t.Errorf("ShardsForWorker(\"\") = %v, want empty", emptyShards)
	}

	// UnassignShard on non-existent shard returns error
	if err := m.UnassignShard(ctx, 99); err == nil {
		t.Fatal("expected error unassigning non-existent shard")
	}

	// UnassignShard on existing shard
	if err := m.UnassignShard(ctx, 1); err != nil {
		t.Fatal(err)
	}
	w1AfterUnassign, _ := m.ShardsForWorker(ctx, "w1")
	if !reflect.DeepEqual(w1AfterUnassign, []int{2}) {
		t.Errorf("w1 after unassign = %v, want [2]", w1AfterUnassign)
	}
}

func TestMemStore_Workers(t *testing.T) {
	ctx := context.Background()
	m := NewMemStore()

	t1 := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 1, 1, 10, 1, 0, 0, time.UTC)

	// First heartbeat creates worker
	if err := m.Heartbeat(ctx, "worker-b", t1); err != nil {
		t.Fatal(err)
	}
	if err := m.Heartbeat(ctx, "worker-a", t1); err != nil {
		t.Fatal(err)
	}

	// Second heartbeat updates HeartbeatAt but preserves StartedAt
	if err := m.Heartbeat(ctx, "worker-a", t2); err != nil {
		t.Fatal(err)
	}

	workers, err := m.ListWorkers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(workers) != 2 {
		t.Fatalf("expected 2 workers, got %d", len(workers))
	}
	// Workers sorted by WorkerID
	if workers[0].WorkerID != "worker-a" || workers[1].WorkerID != "worker-b" {
		t.Errorf("workers not sorted by ID: %v", workers)
	}
	if !workers[0].StartedAt.Equal(t1) {
		t.Errorf("worker-a StartedAt = %v, want %v", workers[0].StartedAt, t1)
	}
	if !workers[0].HeartbeatAt.Equal(t2) {
		t.Errorf("worker-a HeartbeatAt = %v, want %v", workers[0].HeartbeatAt, t2)
	}

	// DeleteWorker
	if err := m.DeleteWorker(ctx, "worker-a"); err != nil {
		t.Fatal(err)
	}
	workersAfter, err := m.ListWorkers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(workersAfter) != 1 || workersAfter[0].WorkerID != "worker-b" {
		t.Errorf("workers after delete: %v", workersAfter)
	}
}

func TestMemStore_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	m := NewMemStore("C1", "C2")

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			workerID := fmt.Sprintf("worker-%d", id)
			now := time.Now()

			_ = m.EnsureShard(ctx, id)
			_ = m.AssignShard(ctx, id, workerID, now)
			_, _ = m.ShardsForWorker(ctx, workerID)
			_ = m.Heartbeat(ctx, workerID, now)
			_, _ = m.ListWorkers(ctx)
			_, _ = m.ListShards(ctx)
			_, _ = m.ListContracts(ctx)
			_ = m.UnassignShard(ctx, id)
			_ = m.DeleteWorker(ctx, workerID)
		}(i)
	}
	wg.Wait()
}
