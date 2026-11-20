package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	replicaLagSecondsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "db", "replica_lag_seconds"),
		"Current replication lag of the read replica database in seconds.",
		nil, nil,
	)
	replicaPoolAcquiresTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "db", "replica_pool_acquires_total"),
		"Cumulative number of successful acquires from the replica Postgres connection pool.",
		nil, nil,
	)
	replicaPoolAcquiredConnsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "db", "replica_pool_acquired_connections"),
		"Connections currently acquired from the replica pool.",
		nil, nil,
	)
	replicaPoolIdleConnsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "db", "replica_pool_idle_connections"),
		"Connections currently idle in the replica pool.",
		nil, nil,
	)
	replicaPoolTotalConnsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "db", "replica_pool_total_connections"),
		"Connections currently held by the replica pool.",
		nil, nil,
	)
)

var (
	replicaLagMu       sync.RWMutex
	replicaLagFunc     func() time.Duration
	replicaPoolMu      sync.RWMutex
	replicaPoolStat    PoolStatFunc
	replicaDescs       = []*prometheus.Desc{
		replicaLagSecondsDesc,
		replicaPoolAcquiresTotalDesc,
		replicaPoolAcquiredConnsDesc,
		replicaPoolIdleConnsDesc,
		replicaPoolTotalConnsDesc,
	}
)

// SetReplicaLag wires a live replication lag function into the metrics collector.
func SetReplicaLag(f func() time.Duration) {
	replicaLagMu.Lock()
	replicaLagFunc = f
	replicaLagMu.Unlock()
}

// SetReplicaPool wires the live replica pool into the metrics collector.
func SetReplicaPool(stat PoolStatFunc) {
	replicaPoolMu.Lock()
	replicaPoolStat = stat
	replicaPoolMu.Unlock()
}

// ReplicaCollector collects replica lag and connection pool metrics.
type ReplicaCollector struct{}

func (ReplicaCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range replicaDescs {
		ch <- d
	}
}

func (ReplicaCollector) Collect(ch chan<- prometheus.Metric) {
	replicaLagMu.RLock()
	rf := replicaLagFunc
	replicaLagMu.RUnlock()
	if rf != nil {
		lag := rf().Seconds()
		if lag < 0 {
			lag = 0
		}
		ch <- prometheus.MustNewConstMetric(replicaLagSecondsDesc, prometheus.GaugeValue, lag)
	}

	replicaPoolMu.RLock()
	stat := replicaPoolStat
	replicaPoolMu.RUnlock()
	if stat != nil {
		s := stat()
		if s != nil {
			ch <- prometheus.MustNewConstMetric(replicaPoolAcquiresTotalDesc, prometheus.CounterValue, float64(s.AcquireCount()))
			ch <- prometheus.MustNewConstMetric(replicaPoolAcquiredConnsDesc, prometheus.GaugeValue, float64(s.AcquiredConns()))
			ch <- prometheus.MustNewConstMetric(replicaPoolIdleConnsDesc, prometheus.GaugeValue, float64(s.IdleConns()))
			ch <- prometheus.MustNewConstMetric(replicaPoolTotalConnsDesc, prometheus.GaugeValue, float64(s.TotalConns()))
		}
	}
}

func init() {
	Registry.MustRegister(ReplicaCollector{})
}
