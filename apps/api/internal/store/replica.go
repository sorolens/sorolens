package store

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type primaryOnlyKey struct{}

// WithPrimaryOnly returns a context that forces queries to execute on the primary database.
func WithPrimaryOnly(ctx context.Context) context.Context {
	return context.WithValue(ctx, primaryOnlyKey{}, true)
}

// IsPrimaryOnly reports whether the context requires primary-only routing.
func IsPrimaryOnly(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	v, _ := ctx.Value(primaryOnlyKey{}).(bool)
	return v
}

// ReplicaLagSampler defines the interface for measuring database replication lag.
type ReplicaLagSampler interface {
	SampleLag(ctx context.Context) (time.Duration, error)
}

// PostgresLagSampler measures replication lag via Postgres replay timestamp.
type PostgresLagSampler struct {
	pool *pgxpool.Pool
}

func NewPostgresLagSampler(pool *pgxpool.Pool) *PostgresLagSampler {
	return &PostgresLagSampler{pool: pool}
}

func (s *PostgresLagSampler) SampleLag(ctx context.Context) (time.Duration, error) {
	if s.pool == nil {
		return 0, nil
	}
	var replayTime *time.Time
	err := s.pool.QueryRow(ctx, "SELECT pg_last_xact_replay_timestamp()").Scan(&replayTime)
	if err != nil {
		return 0, err
	}
	if replayTime == nil {
		return 0, nil
	}
	lag := time.Since(*replayTime)
	if lag < 0 {
		lag = 0
	}
	return lag, nil
}

// ReplicaLagGuard tracks replication health and falls back to primary if lag exceeds threshold.
type ReplicaLagGuard struct {
	mu           sync.RWMutex
	threshold    time.Duration
	currentLag   time.Duration
	lastSampled  time.Time
	sampler      ReplicaLagSampler
	healthy      bool
	forceLagging bool
}

func NewReplicaLagGuard(threshold time.Duration, sampler ReplicaLagSampler) *ReplicaLagGuard {
	if threshold <= 0 {
		threshold = 10 * time.Second
	}
	return &ReplicaLagGuard{
		threshold: threshold,
		sampler:   sampler,
		healthy:   true,
	}
}

func (g *ReplicaLagGuard) CheckLag(ctx context.Context) (time.Duration, bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.forceLagging {
		g.healthy = false
		return g.currentLag, false, nil
	}

	if g.sampler == nil {
		g.healthy = true
		return 0, true, nil
	}

	lag, err := g.sampler.SampleLag(ctx)
	if err != nil {
		g.healthy = false
		return 0, false, err
	}

	g.currentLag = lag
	g.lastSampled = time.Now()
	g.healthy = lag <= g.threshold
	return lag, g.healthy, nil
}

func (g *ReplicaLagGuard) IsHealthy() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.healthy
}

func (g *ReplicaLagGuard) CurrentLag() time.Duration {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.currentLag
}

func (g *ReplicaLagGuard) SetLag(d time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.currentLag = d
	g.healthy = d <= g.threshold
}

func (g *ReplicaLagGuard) SetForceLagging(force bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.forceLagging = force
	if force {
		g.healthy = false
	}
}
