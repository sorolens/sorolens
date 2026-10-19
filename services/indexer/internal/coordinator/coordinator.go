// Package coordinator spreads the indexer across multiple worker processes.
//
// One contract belongs to exactly one shard, chosen by hashing its contract id,
// and each shard is owned by exactly one live worker. A leader — elected with a
// Postgres advisory lock — creates the shard rows, assigns them to live
// workers, and reassigns the shards of any worker whose heartbeat has gone
// stale, so a crashed worker's contracts resume automatically.
//
// The package deliberately depends only on the standard library. Persistence is
// supplied by the caller as a Store, which keeps the sharding algorithm free of
// a database driver and testable in memory.
package coordinator

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"sort"
	"time"
)

const (
	// DefaultShardCount is how many shards contracts are spread across. It is
	// deliberately larger than the expected worker count so reassignment moves
	// small units of work rather than queaking a whole worker's backlog onto
	// one survivor.
	DefaultShardCount = 64

	// DefaultHeartbeatInterval is how often a worker reports liveness.
	DefaultHeartbeatInterval = 5 * time.Second

	// DefaultStaleAfter is how long a worker may miss heartbeats before the
	// leader treats it as dead and reassigns its shards.
	DefaultStaleAfter = 30 * time.Second

	// DefaultRebalanceInterval is how often the leader re-examines ownership.
	DefaultRebalanceInterval = 10 * time.Second

	// advisoryLockKey is the pg_try_advisory_lock key that elects the leader.
	// The value is the ASCII bytes of "Sorolens".
	advisoryLockKey int64 = 0x536F726F6C656E73
)

// Shard is one unit of indexer work: a set of contracts that must be indexed by
// exactly one worker. WorkerID is empty when the shard is unowned.
type Shard struct {
	ShardID    int
	WorkerID   string
	AssignedAt *time.Time
}

// Worker is a live indexer process and the moment it last reported in.
type Worker struct {
	WorkerID    string
	StartedAt   time.Time
	HeartbeatAt time.Time
}

// Store persists shard ownership. Implementations must be safe for concurrent
// use by one leader and many workers.
type Store interface {
	// ListContracts returns every contract id eligible for indexing.
	ListContracts(ctx context.Context) ([]string, error)

	// TryAdvisoryLock reports whether this process acquired the leader lock.
	// It never blocks: a false return means another leader already holds it.
	TryAdvisoryLock(ctx context.Context, key int64) (bool, error)
	// ReleaseAdvisoryLock gives up the leader lock so another process can win.
	ReleaseAdvisoryLock(ctx context.Context, key int64) error

	// EnsureShard creates the shard row if it does not exist, unowned.
	EnsureShard(ctx context.Context, shardID int) error
	// ListShards returns every shard, owned or not.
	ListShards(ctx context.Context) ([]Shard, error)
	// AssignShard records ownership of a shard by a worker.
	AssignShard(ctx context.Context, shardID int, workerID string, at time.Time) error
	// UnassignShard clears ownership, returning the shard to the pool.
	UnassignShard(ctx context.Context, shardID int) error
	// ShardsForWorker lists the shard ids currently owned by one worker.
	ShardsForWorker(ctx context.Context, workerID string) ([]int, error)

	// Heartbeat records that a worker is alive at the given time.
	Heartbeat(ctx context.Context, workerID string, at time.Time) error
	// ListWorkers returns every known worker, live or stale.
	ListWorkers(ctx context.Context) ([]Worker, error)
	// DeleteWorker forgets a worker whose shards have been reassigned.
	DeleteWorker(ctx context.Context, workerID string) error
}

// Config tunes the coordinator and worker loops. The zero value is usable.
type Config struct {
	// ShardCount is the total number of shards. Defaults to DefaultShardCount.
	ShardCount int
	// WorkerID identifies this process. Required for a worker role.
	WorkerID string
	// HeartbeatInterval is how often a worker reports liveness.
	HeartbeatInterval time.Duration
	// StaleAfter is how long a worker may go unheard before its shards move.
	StaleAfter time.Duration
	// RebalanceInterval is how often the leader reconciles ownership.
	RebalanceInterval time.Duration

	// now is injectable so tests control staleness without sleeping.
	now func() time.Time
}

func (c Config) withDefaults() Config {
	if c.ShardCount <= 0 {
		c.ShardCount = DefaultShardCount
	}
	if c.HeartbeatInterval <= 0 {
		c.HeartbeatInterval = DefaultHeartbeatInterval
	}
	if c.StaleAfter <= 0 {
		c.StaleAfter = DefaultStaleAfter
	}
	if c.RebalanceInterval <= 0 {
		c.RebalanceInterval = DefaultRebalanceInterval
	}
	if c.now == nil {
		c.now = time.Now
	}
	return c
}

// ShardForContract maps a contract id onto a shard. The mapping is stable
// across processes and restarts, which is what lets any worker recompute the
// same ownership from the same inputs.
func ShardForContract(contractID string, shardCount int) int {
	if shardCount <= 0 {
		return 0
	}
	h := fnv.New64a()
	// hash.Hash never returns an error for Write; the contract is documented.
	_, _ = h.Write([]byte(contractID))
	return int(h.Sum64() % uint64(shardCount))
}

// ContractsForShards filters contracts down to those owned by the given
// shards, sorted so a worker indexes a deterministic set each pass.
func ContractsForShards(contracts []string, shards []int, shardCount int) []string {
	if len(contracts) == 0 || len(shards) == 0 {
		return nil
	}
	want := make(map[int]struct{}, len(shards))
	for _, s := range shards {
		want[s] = struct{}{}
	}
	out := make([]string, 0, len(contracts))
	for _, c := range contracts {
		if _, ok := want[ShardForContract(c, shardCount)]; ok {
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out
}

// Coordinator is the leader side: it owns the shard table's contents.
type Coordinator struct {
	store Store
	log   *slog.Logger
	cfg   Config
}

// New returns a Coordinator. A nil logger is replaced with a discard logger.
func New(store Store, log *slog.Logger, cfg Config) *Coordinator {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Coordinator{store: store, log: log, cfg: cfg.withDefaults()}
}

// EnsureShards creates any shard rows that do not exist yet, so a fresh
// deployment has the full set to hand out.
func (c *Coordinator) EnsureShards(ctx context.Context) error {
	existing, err := c.store.ListShards(ctx)
	if err != nil {
		return fmt.Errorf("list shards: %w", err)
	}
	have := make(map[int]struct{}, len(existing))
	for _, s := range existing {
		have[s.ShardID] = struct{}{}
	}
	for id := 0; id < c.cfg.ShardCount; id++ {
		if _, ok := have[id]; ok {
			continue
		}
		if err := c.store.EnsureShard(ctx, id); err != nil {
			return fmt.Errorf("ensure shard %d: %w", id, err)
		}
	}
	return nil
}

// Rebalance reconciles shard ownership with the set of live workers and
// returns how many shards changed hands.
//
// A worker is live when its last heartbeat is within StaleAfter. Shards owned
// by a stale worker are handed out again, which is what bounds recovery to
// roughly StaleAfter + RebalanceInterval after a crash. With no live workers
// every shard is unassigned rather than left pointing at a dead owner.
func (c *Coordinator) Rebalance(ctx context.Context) (int, error) {
	now := c.cfg.now()

	workers, err := c.store.ListWorkers(ctx)
	if err != nil {
		return 0, fmt.Errorf("list workers: %w", err)
	}

	live := make(map[string]struct{}, len(workers))
	for _, w := range workers {
		if w.WorkerID == "" {
			continue
		}
		if now.Sub(w.HeartbeatAt) <= c.cfg.StaleAfter {
			live[w.WorkerID] = struct{}{}
			continue
		}
		if err := c.store.DeleteWorker(ctx, w.WorkerID); err != nil {
			return 0, fmt.Errorf("delete stale worker %s: %w", w.WorkerID, err)
		}
		c.log.Warn("coordinator: worker is stale, its shards will be reassigned",
			"worker_id", w.WorkerID, "last_heartbeat", w.HeartbeatAt)
	}

	// Sorted so every leader computes the same assignment from the same state.
	ids := make([]string, 0, len(live))
	for id := range live {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	shards, err := c.store.ListShards(ctx)
	if err != nil {
		return 0, fmt.Errorf("list shards: %w", err)
	}
	sort.Slice(shards, func(i, j int) bool { return shards[i].ShardID < shards[j].ShardID })

	moved := 0
	next := 0
	for _, sh := range shards {
		// A shard already owned by a live worker stays put; rebalancing only
		// touches orphaned work, so a healthy worker is never interrupted.
		if sh.WorkerID != "" {
			if _, ok := live[sh.WorkerID]; ok {
				continue
			}
		}
		if len(ids) == 0 {
			if sh.WorkerID != "" {
				if err := c.store.UnassignShard(ctx, sh.ShardID); err != nil {
					return moved, fmt.Errorf("unassign shard %d: %w", sh.ShardID, err)
				}
				moved++
			}
			continue
		}
		target := ids[next%len(ids)]
		next++
		if sh.WorkerID == target {
			continue
		}
		if err := c.store.AssignShard(ctx, sh.ShardID, target, now); err != nil {
			return moved, fmt.Errorf("assign shard %d to %s: %w", sh.ShardID, target, err)
		}
		moved++
	}
	return moved, nil
}

// Run drives the leader loop until ctx is cancelled: it competes for the
// advisory lock, then reconciles ownership on every tick. Losing the lock is
// not an error — it simply means another process is leading.
func (c *Coordinator) Run(ctx context.Context) error {
	ticker := time.NewTicker(c.cfg.RebalanceInterval)
	defer ticker.Stop()

	held := false
	for {
		if !held {
			ok, err := c.store.TryAdvisoryLock(ctx, advisoryLockKey)
			switch {
			case err != nil:
				c.log.Error("coordinator: advisory lock", "err", err)
			case ok:
				held = true
				c.log.Info("coordinator: became leader", "worker_id", c.cfg.WorkerID)
				if err := c.EnsureShards(ctx); err != nil {
					c.log.Error("coordinator: ensure shards", "err", err)
				}
			default:
				c.log.Debug("coordinator: another process holds the leader lock")
			}
		}

		if held {
			moved, err := c.Rebalance(ctx)
			if err != nil {
				c.log.Error("coordinator: rebalance", "err", err)
			} else if moved > 0 {
				c.log.Info("coordinator: reassigned shards", "count", moved)
			}
		}

		select {
		case <-ctx.Done():
			if held {
				// Uses a detached context: ctx is already cancelled, and the
				// lock must still be released or no other leader can take over.
				if err := c.store.ReleaseAdvisoryLock(context.WithoutCancel(ctx), advisoryLockKey); err != nil {
					c.log.Error("coordinator: release leader lock", "err", err)
				}
			}
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// WorkerRunner is the worker side: it heartbeats and asks which contracts it
// owns. It never writes shard assignments — only the leader does that.
type WorkerRunner struct {
	store Store
	log   *slog.Logger
	cfg   Config
}

// NewWorker returns a WorkerRunner. A nil logger is replaced with a discard
// logger. Config.WorkerID must be set.
func NewWorker(store Store, log *slog.Logger, cfg Config) *WorkerRunner {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &WorkerRunner{store: store, log: log, cfg: cfg.withDefaults()}
}

// ErrNoWorkerID is returned when a worker is constructed without an id, which
// would make every replica claim the same shards.
var ErrNoWorkerID = errors.New("coordinator: WorkerID is required for a worker")

// Heartbeat records this worker as alive.
func (w *WorkerRunner) Heartbeat(ctx context.Context) error {
	if w.cfg.WorkerID == "" {
		return ErrNoWorkerID
	}
	return w.store.Heartbeat(ctx, w.cfg.WorkerID, w.cfg.now())
}

// AssignedContracts resolves this worker's shards into the contract ids to
// index, recomputed from the shard table plus the contract list so no extra
// per-worker state has to be stored.
func (w *WorkerRunner) AssignedContracts(ctx context.Context) ([]string, error) {
	if w.cfg.WorkerID == "" {
		return nil, ErrNoWorkerID
	}
	shards, err := w.store.ShardsForWorker(ctx, w.cfg.WorkerID)
	if err != nil {
		return nil, fmt.Errorf("shards for worker: %w", err)
	}
	if len(shards) == 0 {
		return nil, nil
	}
	contracts, err := w.store.ListContracts(ctx)
	if err != nil {
		return nil, fmt.Errorf("list contracts: %w", err)
	}
	return ContractsForShards(contracts, shards, w.cfg.ShardCount), nil
}

// Run heartbeats and indexes this worker's contracts until ctx is cancelled.
// onAssigned receives the contracts this worker owns on each tick; a pass that
// returns an error is logged and retried on the next tick rather than fatal,
// so one bad contract cannot take the worker down.
func (w *WorkerRunner) Run(ctx context.Context, onAssigned func(context.Context, []string) error) error {
	if w.cfg.WorkerID == "" {
		return ErrNoWorkerID
	}

	// Heartbeats run on their own goroutine. A pass can outlive the heartbeat
	// interval, and if liveness were reported inline the leader would declare
	// this worker dead mid-pass and hand its shards to someone else — two
	// workers indexing the same contracts.
	hbCtx, stopHeartbeat := context.WithCancel(ctx)
	defer stopHeartbeat()
	go func() {
		ticker := time.NewTicker(w.cfg.HeartbeatInterval)
		defer ticker.Stop()
		for {
			if err := w.Heartbeat(hbCtx); err != nil && hbCtx.Err() == nil {
				w.log.Error("worker: heartbeat failed", "worker_id", w.cfg.WorkerID, "err", err)
			}
			select {
			case <-hbCtx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	for {
		contracts, err := w.AssignedContracts(ctx)
		switch {
		case err != nil && !errors.Is(err, context.Canceled):
			w.log.Error("worker: resolve assigned contracts", "worker_id", w.cfg.WorkerID, "err", err)
		case len(contracts) == 0:
			w.log.Debug("worker: no shards assigned yet", "worker_id", w.cfg.WorkerID)
		case onAssigned != nil:
			if err := onAssigned(ctx, contracts); err != nil {
				w.log.Error("worker: index pass", "worker_id", w.cfg.WorkerID, "err", err)
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(w.cfg.HeartbeatInterval):
		}
	}
}
