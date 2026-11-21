# 0003. Horizontally-scaled indexer with worker sharding

## Status

Accepted (retroactive) — 2026-09-25

## Context

A single-process indexer cannot keep up with head when tracking 100+ contracts under mainnet traffic. The indexer required horizontal scaling without duplicate polling or split-brain coordination overhead.

## Decision

Implement worker sharding coordinated via Postgres:

1. **Static shard count:** Cluster shard count is fixed at deployment (defaulting to 64 shards).
2. **Deterministic derivation:** Contract-to-shard mapping is a pure function:
   `shard_id = fnv1a64(contract_id) % shard_count` (`coordinator.ShardForContract`). Adding contracts requires no writes to the shard table.
3. **Leader election:** Coordinator replicas elect one leader via a session-scoped Postgres advisory lock (`pg_try_advisory_lock(0x536F726F6C656E73)`). Only the leader writes `indexer_shards`.
4. **Liveness and rebalance:** Workers heartbeat into `indexer_workers`. The leader checks heartbeats every 10s. If a worker goes stale past `--worker-stale-after` (30s), its shards are distributed round-robin across live workers. Total recovery is bounded by 40s (`worker-stale-after + rebalance-interval`). Stable workers retain assignments without churn.

## Consequences

- **Positive:** Horizontal scaling across worker replicas; deterministic shard assignments without distributed consensus clusters (Raft/ZooKeeper); crash recovery bounded by 40s; contract progress durable in `sync_state`.
- **Negative:** Shard count cannot be altered dynamically while workers run; uneven contract volume is not automatically reweighted across shards.

## Alternatives considered

- Message queue task distribution: Rejected to avoid adding operational dependencies beyond Postgres.
- Dynamic consistent hashing: Avoided in favor of static FNV-1a modulo hashing for predictable zero-coordination assignment.

## Sources

- Issue: [#272](https://github.com/sorolens/sorolens/issues/272)
- Pull requests: [#455](https://github.com/sorolens/sorolens/pull/455) (commit `91f1274`), [#672](https://github.com/sorolens/sorolens/pull/672) (commit `58abbc7`)
- Migration: [`000030_indexer_shards.up.sql`](https://github.com/sorolens/sorolens/blob/main/apps/api/internal/db/migrations/000030_indexer_shards.up.sql)
- Code: [`services/indexer/internal/coordinator/coordinator.go`](https://github.com/sorolens/sorolens/blob/main/services/indexer/internal/coordinator/coordinator.go), [`docs/indexer-sharding.md`](https://github.com/sorolens/sorolens/blob/main/docs/indexer-sharding.md)

## Open questions for maintainers

- Rationale not documented for fixing the default shard count at 64, or how live resharding should be performed if a cluster grows beyond 64 shards.
