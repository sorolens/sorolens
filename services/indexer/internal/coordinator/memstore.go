package coordinator

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// MemStore is an in-memory Store used by tests and by local runs of the
// coordinator role. It models one advisory lock held by at most one caller,
// matching pg_try_advisory_lock semantics.
//
// It is not durable and not shared between processes: a real deployment needs
// a Postgres-backed Store (see docs/indexer-sharding.md for the schema).
type MemStore struct {
	mu        sync.Mutex
	contracts []string
	shards    map[int]*Shard
	workers   map[string]Worker
	lockKey   *int64
}

// NewMemStore returns a MemStore over the given contracts.
func NewMemStore(contracts ...string) *MemStore {
	return &MemStore{
		contracts: append([]string(nil), contracts...),
		shards:    make(map[int]*Shard),
		workers:   make(map[string]Worker),
	}
}

// SetContracts replaces the contract set.
func (m *MemStore) SetContracts(contracts []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.contracts = append([]string(nil), contracts...)
}

// ListContracts returns the contract ids eligible for indexing.
func (m *MemStore) ListContracts(context.Context) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := append([]string(nil), m.contracts...)
	sort.Strings(out)
	return out, nil
}

// TryAdvisoryLock acquires the single advisory lock, returning false when
// another caller already holds it.
func (m *MemStore) TryAdvisoryLock(_ context.Context, key int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.lockKey != nil {
		return *m.lockKey == key, nil
	}
	k := key
	m.lockKey = &k
	return true, nil
}

// ReleaseAdvisoryLock gives the advisory lock back.
func (m *MemStore) ReleaseAdvisoryLock(_ context.Context, key int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.lockKey != nil && *m.lockKey == key {
		m.lockKey = nil
	}
	return nil
}

// EnsureShard creates the shard if it does not exist.
func (m *MemStore) EnsureShard(_ context.Context, shardID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.shards[shardID]; !ok {
		m.shards[shardID] = &Shard{ShardID: shardID}
	}
	return nil
}

// ListShards returns every shard, ordered by id.
func (m *MemStore) ListShards(context.Context) ([]Shard, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Shard, 0, len(m.shards))
	for _, s := range m.shards {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ShardID < out[j].ShardID })
	return out, nil
}

// AssignShard records ownership of a shard.
func (m *MemStore) AssignShard(_ context.Context, shardID int, workerID string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.shards[shardID]
	if !ok {
		return fmt.Errorf("shard %d does not exist", shardID)
	}
	t := at
	s.WorkerID = workerID
	s.AssignedAt = &t
	return nil
}

// UnassignShard clears ownership of a shard.
func (m *MemStore) UnassignShard(_ context.Context, shardID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.shards[shardID]
	if !ok {
		return fmt.Errorf("shard %d does not exist", shardID)
	}
	s.WorkerID = ""
	s.AssignedAt = nil
	return nil
}

// ShardsForWorker lists the shard ids owned by one worker.
func (m *MemStore) ShardsForWorker(_ context.Context, workerID string) ([]int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []int
	for _, s := range m.shards {
		if s.WorkerID == workerID && workerID != "" {
			out = append(out, s.ShardID)
		}
	}
	sort.Ints(out)
	return out, nil
}

// Heartbeat records a worker as alive, creating it on first contact.
func (m *MemStore) Heartbeat(_ context.Context, workerID string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	w, ok := m.workers[workerID]
	if !ok {
		w = Worker{WorkerID: workerID, StartedAt: at}
	}
	w.HeartbeatAt = at
	m.workers[workerID] = w
	return nil
}

// ListWorkers returns every known worker.
func (m *MemStore) ListWorkers(context.Context) ([]Worker, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Worker, 0, len(m.workers))
	for _, w := range m.workers {
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].WorkerID < out[j].WorkerID })
	return out, nil
}

// DeleteWorker forgets a worker.
func (m *MemStore) DeleteWorker(_ context.Context, workerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.workers, workerID)
	return nil
}
