# Read-Replica Routing and Replication-Lag Guard

Sorolens supports routing read-only queries to a dedicated PostgreSQL read replica when configured, while sending all mutating queries and read-after-write flows to the primary database.

## Configuration

The read pool is activated by setting the `DATABASE_READ_URL` environment variable:

- `DATABASE_URL`: Primary PostgreSQL connection string (receives writes and fallback reads).
- `DATABASE_READ_URL`: (Optional) Read replica connection string. When unset, all queries route through the single primary connection pool.
- `DATABASE_REPLICA_LAG_THRESHOLD`: (Optional, default `10s`) Maximum allowable replication lag duration before the API temporarily falls back to routing reads to the primary database.

## Replication-Lag Guard

When `DATABASE_READ_URL` is set, `ReplicaLagGuard` samples the replica's replication lag:
- The guard inspects `pg_last_xact_replay_timestamp()` against current time.
- If the replication lag exceeds `DATABASE_REPLICA_LAG_THRESHOLD` (or if replication health checks fail), read traffic is automatically directed to the primary pool until lag drops below the threshold.
- To guarantee read-your-writes consistency, callers can wrap request contexts with `store.WithPrimaryOnly(ctx)` to bypass the read replica entirely.

## Observability & Metrics

Prometheus metrics are exposed via the standard `/metrics` endpoint:
- `sorolens_db_replica_lag_seconds`: Current replication lag duration in seconds.
- `sorolens_db_replica_pool_acquires_total`: Cumulative acquires from the replica connection pool.
- `sorolens_db_replica_pool_acquired_connections`: Active connections acquired from the replica pool.
- `sorolens_db_replica_pool_idle_connections`: Idle connections in the replica pool.
- `sorolens_db_replica_pool_total_connections`: Total connections in the replica pool.
