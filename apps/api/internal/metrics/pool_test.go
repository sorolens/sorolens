package metrics

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// fakePoolStat is a fixed snapshot with one distinct value per method, so a
// swapped or miswired field fails the exact-match test.
type fakePoolStat struct{}

func (fakePoolStat) AcquireCount() int64            { return 42 }
func (fakePoolStat) AcquireDuration() time.Duration { return 1250 * time.Millisecond }
func (fakePoolStat) AcquiredConns() int32           { return 3 }
func (fakePoolStat) CanceledAcquireCount() int64    { return 2 }
func (fakePoolStat) ConstructingConns() int32       { return 1 }
func (fakePoolStat) EmptyAcquireCount() int64       { return 5 }
func (fakePoolStat) EmptyAcquireWaitTime() time.Duration {
	return 300 * time.Millisecond
}
func (fakePoolStat) IdleConns() int32               { return 4 }
func (fakePoolStat) MaxConns() int32                { return 10 }
func (fakePoolStat) TotalConns() int32              { return 8 }
func (fakePoolStat) NewConnsCount() int64           { return 9 }
func (fakePoolStat) MaxLifetimeDestroyCount() int64 { return 6 }
func (fakePoolStat) MaxIdleDestroyCount() int64     { return 1 }

// TestPoolCollectorExportsSnapshot pins every series name, type and value the
// collector derives from the pool snapshot.
func TestPoolCollectorExportsSnapshot(t *testing.T) {
	c := &poolCollector{stat: func() PoolStat { return fakePoolStat{} }}

	const want = `
# HELP sorolens_db_pool_acquire_duration_seconds_total Cumulative time spent waiting for successful acquires from the Postgres connection pool, in seconds.
# TYPE sorolens_db_pool_acquire_duration_seconds_total counter
sorolens_db_pool_acquire_duration_seconds_total 1.25
# HELP sorolens_db_pool_acquired_connections Connections currently acquired from the pool.
# TYPE sorolens_db_pool_acquired_connections gauge
sorolens_db_pool_acquired_connections 3
# HELP sorolens_db_pool_acquires_total Cumulative number of successful acquires from the Postgres connection pool.
# TYPE sorolens_db_pool_acquires_total counter
sorolens_db_pool_acquires_total 42
# HELP sorolens_db_pool_canceled_acquires_total Cumulative acquires canceled by a context before a connection was assigned.
# TYPE sorolens_db_pool_canceled_acquires_total counter
sorolens_db_pool_canceled_acquires_total 2
# HELP sorolens_db_pool_constructing_connections Connections the pool is currently constructing.
# TYPE sorolens_db_pool_constructing_connections gauge
sorolens_db_pool_constructing_connections 1
# HELP sorolens_db_pool_connections_destroyed_idle_total Cumulative connections destroyed after exceeding the maximum connection idle time.
# TYPE sorolens_db_pool_connections_destroyed_idle_total counter
sorolens_db_pool_connections_destroyed_idle_total 1
# HELP sorolens_db_pool_connections_destroyed_lifetime_total Cumulative connections destroyed after exceeding the maximum connection lifetime.
# TYPE sorolens_db_pool_connections_destroyed_lifetime_total counter
sorolens_db_pool_connections_destroyed_lifetime_total 6
# HELP sorolens_db_pool_empty_acquire_wait_seconds_total Cumulative time spent waiting for a connection because the pool was empty, in seconds.
# TYPE sorolens_db_pool_empty_acquire_wait_seconds_total counter
sorolens_db_pool_empty_acquire_wait_seconds_total 0.3
# HELP sorolens_db_pool_empty_acquires_total Cumulative acquires that had to wait because the pool held no idle connection.
# TYPE sorolens_db_pool_empty_acquires_total counter
sorolens_db_pool_empty_acquires_total 5
# HELP sorolens_db_pool_idle_connections Connections currently idle in the pool.
# TYPE sorolens_db_pool_idle_connections gauge
sorolens_db_pool_idle_connections 4
# HELP sorolens_db_pool_max_connections Maximum number of connections the pool may open.
# TYPE sorolens_db_pool_max_connections gauge
sorolens_db_pool_max_connections 10
# HELP sorolens_db_pool_new_connections_total Cumulative number of new connections opened by the pool.
# TYPE sorolens_db_pool_new_connections_total counter
sorolens_db_pool_new_connections_total 9
# HELP sorolens_db_pool_total_connections Connections currently held by the pool (acquired, idle and constructing).
# TYPE sorolens_db_pool_total_connections gauge
sorolens_db_pool_total_connections 8
`
	if err := testutil.CollectAndCompare(c, strings.NewReader(want)); err != nil {
		t.Errorf("pool collector exposition mismatch: %v", err)
	}
}

// TestPoolCollectorUnwiredEmitsNothing pins the behaviour before SetPool:
// the collector stays registered but reports no series rather than zeros
// that would look like a drained pool.
func TestPoolCollectorUnwiredEmitsNothing(t *testing.T) {
	SetPool(nil) // no pool wired for this assertion
	if got := testutil.CollectAndCount(&poolCollector{}); got != 0 {
		t.Errorf("unwired pool collector emitted %d metrics, want 0", got)
	}
}

// TestPoolCollectorSeriesAreLabelFree runs the collector through a pedantic
// registry (which cross-checks Describe against Collect) and asserts every
// series carries no labels at all — the strongest possible bound on label
// cardinality.
func TestPoolCollectorSeriesAreLabelFree(t *testing.T) {
	SetPool(func() PoolStat { return fakePoolStat{} })
	t.Cleanup(func() { SetPool(nil) })

	reg := prometheus.NewPedanticRegistry()
	if err := reg.Register(PoolCollector()); err != nil {
		t.Fatalf("register pool collector: %v", err)
	}
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	total := 0
	for _, family := range families {
		if !strings.HasPrefix(family.GetName(), "sorolens_db_pool_") {
			t.Errorf("unexpected metric family %q", family.GetName())
			continue
		}
		for _, m := range family.Metric {
			total++
			if len(m.Label) != 0 {
				t.Errorf("%s carries %d labels, want 0", family.GetName(), len(m.Label))
			}
		}
	}
	if want := len(poolDescs); total != want {
		t.Errorf("gathered %d pool series, want %d", total, want)
	}
}

// TestRegistryServesPoolStats verifies the METRICS_PORT admin listener's
// registry exports pool stats once a pool is wired.
func TestRegistryServesPoolStats(t *testing.T) {
	SetPool(func() PoolStat { return fakePoolStat{} })
	t.Cleanup(func() { SetPool(nil) })

	families, err := Registry.Gather()
	if err != nil {
		t.Fatalf("gather registry: %v", err)
	}
	found := false
	for _, family := range families {
		if family.GetName() == "sorolens_db_pool_max_connections" {
			found = true
		}
	}
	if !found {
		t.Errorf("Registry is missing sorolens_db_pool_max_connections")
	}
}
