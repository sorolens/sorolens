package store

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type mockLagSampler struct {
	lag time.Duration
	err error
}

func (m *mockLagSampler) SampleLag(ctx context.Context) (time.Duration, error) {
	return m.lag, m.err
}

func TestWithPrimaryOnly(t *testing.T) {
	ctx := context.Background()
	if IsPrimaryOnly(ctx) {
		t.Errorf("expected IsPrimaryOnly to be false for background context")
	}

	forced := WithPrimaryOnly(ctx)
	if !IsPrimaryOnly(forced) {
		t.Errorf("expected IsPrimaryOnly to be true after WithPrimaryOnly")
	}
}

func TestReplicaLagGuard_Health(t *testing.T) {
	sampler := &mockLagSampler{lag: 2 * time.Second}
	guard := NewReplicaLagGuard(5*time.Second, sampler)

	lag, healthy, err := guard.CheckLag(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !healthy || lag != 2*time.Second {
		t.Errorf("expected healthy=true, lag=2s; got healthy=%v, lag=%v", healthy, lag)
	}
	if !guard.IsHealthy() {
		t.Errorf("expected IsHealthy()=true")
	}

	// Update lag beyond threshold
	sampler.lag = 12 * time.Second
	lag, healthy, err = guard.CheckLag(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if healthy || lag != 12*time.Second {
		t.Errorf("expected healthy=false, lag=12s; got healthy=%v, lag=%v", healthy, lag)
	}
	if guard.IsHealthy() {
		t.Errorf("expected IsHealthy()=false")
	}

	// Manual override / set lag
	guard.SetLag(1 * time.Second)
	if !guard.IsHealthy() {
		t.Errorf("expected healthy=true after SetLag(1s)")
	}

	guard.SetForceLagging(true)
	if guard.IsHealthy() {
		t.Errorf("expected healthy=false after SetForceLagging(true)")
	}
}

func TestPostgresStore_ReadDBRouting(t *testing.T) {
	// Dummy pointers for identity comparison
	primaryPool := &pgxpool.Pool{}
	readPool := &pgxpool.Pool{}

	guard := NewReplicaLagGuard(5*time.Second, &mockLagSampler{lag: 1 * time.Second})
	_, _, _ = guard.CheckLag(context.Background())

	s := &postgresStore{
		pool:            primaryPool,
		readPool:        readPool,
		replicaLagGuard: guard,
	}

	// 1. Healthy replica -> readPool
	if db := s.readDB(context.Background()); db != readPool {
		t.Errorf("expected readPool when healthy, got %p", db)
	}

	// 2. WithPrimaryOnly -> primaryPool
	primaryCtx := WithPrimaryOnly(context.Background())
	if db := s.readDB(primaryCtx); db != primaryPool {
		t.Errorf("expected primaryPool with WithPrimaryOnly, got %p", db)
	}

	// 3. Lagging replica -> fallback to primaryPool
	guard.SetLag(10 * time.Second)
	if db := s.readDB(context.Background()); db != primaryPool {
		t.Errorf("expected fallback to primaryPool when lagging, got %p", db)
	}

	// 4. When readPool is nil -> always primaryPool
	noReplicaStore := &postgresStore{pool: primaryPool}
	if db := noReplicaStore.readDB(context.Background()); db != primaryPool {
		t.Errorf("expected primaryPool when readPool is nil, got %p", db)
	}
}
