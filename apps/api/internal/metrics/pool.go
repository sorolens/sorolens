package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// PoolStat is the read-only snapshot of a Postgres connection pool that the
// pool collector exports. It is satisfied by *pgxpool.Stat — the value
// returned by (*pgxpool.Pool).Stat() — so tests can supply a fake without a
// database.
type PoolStat interface {
	AcquireCount() int64
	AcquireDuration() time.Duration
	AcquiredConns() int32
	CanceledAcquireCount() int64
	ConstructingConns() int32
	EmptyAcquireCount() int64
	EmptyAcquireWaitTime() time.Duration
	IdleConns() int32
	MaxConns() int32
	TotalConns() int32
	NewConnsCount() int64
	MaxLifetimeDestroyCount() int64
	MaxIdleDestroyCount() int64
}

// PoolStatFunc returns the current pool statistics snapshot. Wire a live
// pool with a closure: metrics.SetPool(func() metrics.PoolStat { return pool.Stat() }).
type PoolStatFunc func() PoolStat

// Pool metric descriptors. Every series is label-free: a pool has one fixed
// set of series no matter how much traffic the API serves, so label
// cardinality is bounded by construction.
var (
	poolAcquiresTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "db", "pool_acquires_total"),
		"Cumulative number of successful acquires from the Postgres connection pool.",
		nil, nil,
	)
	poolAcquireDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "db", "pool_acquire_duration_seconds_total"),
		"Cumulative time spent waiting for successful acquires from the Postgres connection pool, in seconds.",
		nil, nil,
	)
	poolEmptyAcquiresDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "db", "pool_empty_acquires_total"),
		"Cumulative acquires that had to wait because the pool held no idle connection.",
		nil, nil,
	)
	poolEmptyAcquireWaitDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "db", "pool_empty_acquire_wait_seconds_total"),
		"Cumulative time spent waiting for a connection because the pool was empty, in seconds.",
		nil, nil,
	)
	poolCanceledAcquiresDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "db", "pool_canceled_acquires_total"),
		"Cumulative acquires canceled by a context before a connection was assigned.",
		nil, nil,
	)
	poolNewConnsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "db", "pool_new_connections_total"),
		"Cumulative number of new connections opened by the pool.",
		nil, nil,
	)
	poolDestroyedLifetimeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "db", "pool_connections_destroyed_lifetime_total"),
		"Cumulative connections destroyed after exceeding the maximum connection lifetime.",
		nil, nil,
	)
	poolDestroyedIdleDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "db", "pool_connections_destroyed_idle_total"),
		"Cumulative connections destroyed after exceeding the maximum connection idle time.",
		nil, nil,
	)
	poolAcquiredConnsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "db", "pool_acquired_connections"),
		"Connections currently acquired from the pool.",
		nil, nil,
	)
	poolIdleConnsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "db", "pool_idle_connections"),
		"Connections currently idle in the pool.",
		nil, nil,
	)
	poolConstructingConnsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "db", "pool_constructing_connections"),
		"Connections the pool is currently constructing.",
		nil, nil,
	)
	poolTotalConnsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "db", "pool_total_connections"),
		"Connections currently held by the pool (acquired, idle and constructing).",
		nil, nil,
	)
	poolMaxConnsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "db", "pool_max_connections"),
		"Maximum number of connections the pool may open.",
		nil, nil,
	)
)

// poolDescs lists every pool descriptor once; Describe and the series-count
// assertion in the tests both derive from it.
var poolDescs = []*prometheus.Desc{
	poolAcquiresTotalDesc,
	poolAcquireDurationDesc,
	poolEmptyAcquiresDesc,
	poolEmptyAcquireWaitDesc,
	poolCanceledAcquiresDesc,
	poolNewConnsDesc,
	poolDestroyedLifetimeDesc,
	poolDestroyedIdleDesc,
	poolAcquiredConnsDesc,
	poolIdleConnsDesc,
	poolConstructingConnsDesc,
	poolTotalConnsDesc,
	poolMaxConnsDesc,
}

// poolMu guards poolStat, the process-wide snapshot source.
var (
	poolMu   sync.RWMutex
	poolStat PoolStatFunc
)

// SetPool wires the live pgx pool into the process-wide pool collector. That
// collector is registered once on Registry (the METRICS_PORT admin listener)
// and again by the router on its own registry (GET /metrics on the main
// port), so a single call here exports pool stats from every metrics
// endpoint in the process. Passing nil unwires it; scrapes then report no
// pool series at all.
func SetPool(stat PoolStatFunc) {
	poolMu.Lock()
	poolStat = stat
	poolMu.Unlock()
}

// PoolCollector returns the process-wide Postgres pool collector. It is
// registered on every registry that serves /metrics; until SetPool is called
// it emits no series.
func PoolCollector() prometheus.Collector { return dbPoolCollector }

// dbPoolCollector is the process-wide instance; SetPool swaps the source it
// reads.
var dbPoolCollector = &poolCollector{}

// poolCollector reads a snapshot from the pool on every scrape instead of
// sampling on a timer, so the exposition always reflects the pool's current
// state and no background goroutine is needed.
type poolCollector struct {
	// stat is this instance's snapshot source. The process-wide instance
	// leaves it nil and falls back to the package-level source that
	// SetPool updates; tests build instances with an explicit source.
	stat PoolStatFunc
}

// source returns the snapshot function to read on this scrape.
func (c *poolCollector) source() PoolStatFunc {
	if c.stat != nil {
		return c.stat
	}
	poolMu.RLock()
	defer poolMu.RUnlock()
	return poolStat
}

func (c *poolCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range poolDescs {
		ch <- d
	}
}

func (c *poolCollector) Collect(ch chan<- prometheus.Metric) {
	stat := c.source()
	if stat == nil {
		// No pool wired (tests, or an entrypoint that failed to connect):
		// report nothing rather than a misleading zero that looks like a
		// drained pool.
		return
	}
	s := stat()
	if s == nil {
		return
	}

	ch <- prometheus.MustNewConstMetric(poolAcquiresTotalDesc, prometheus.CounterValue, float64(s.AcquireCount()))
	ch <- prometheus.MustNewConstMetric(poolAcquireDurationDesc, prometheus.CounterValue, s.AcquireDuration().Seconds())
	ch <- prometheus.MustNewConstMetric(poolEmptyAcquiresDesc, prometheus.CounterValue, float64(s.EmptyAcquireCount()))
	ch <- prometheus.MustNewConstMetric(poolEmptyAcquireWaitDesc, prometheus.CounterValue, s.EmptyAcquireWaitTime().Seconds())
	ch <- prometheus.MustNewConstMetric(poolCanceledAcquiresDesc, prometheus.CounterValue, float64(s.CanceledAcquireCount()))
	ch <- prometheus.MustNewConstMetric(poolNewConnsDesc, prometheus.CounterValue, float64(s.NewConnsCount()))
	ch <- prometheus.MustNewConstMetric(poolDestroyedLifetimeDesc, prometheus.CounterValue, float64(s.MaxLifetimeDestroyCount()))
	ch <- prometheus.MustNewConstMetric(poolDestroyedIdleDesc, prometheus.CounterValue, float64(s.MaxIdleDestroyCount()))
	ch <- prometheus.MustNewConstMetric(poolAcquiredConnsDesc, prometheus.GaugeValue, float64(s.AcquiredConns()))
	ch <- prometheus.MustNewConstMetric(poolIdleConnsDesc, prometheus.GaugeValue, float64(s.IdleConns()))
	ch <- prometheus.MustNewConstMetric(poolConstructingConnsDesc, prometheus.GaugeValue, float64(s.ConstructingConns()))
	ch <- prometheus.MustNewConstMetric(poolTotalConnsDesc, prometheus.GaugeValue, float64(s.TotalConns()))
	ch <- prometheus.MustNewConstMetric(poolMaxConnsDesc, prometheus.GaugeValue, float64(s.MaxConns()))
}
