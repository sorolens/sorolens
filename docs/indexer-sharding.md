# Sharded indexer

Issue: [#272](https://github.com/sorolens/sorolens/issues/272)

The indexer starts as a single process and has to stay correct when it does not
fit on one machine. This document describes the sharded topology, how a worker
failure recovers, and what an operator must configure.

## Model

Three ideas, and nothing else:

1. **Shard** — a fixed bucket of work. A deployment picks a shard count (64 by
   default) and never changes it while workers are running.
2. **Ownership** — every shard belongs to at most one live worker. Ownership
   lives in `indexer_shards` and is only ever written by the leader.
3. **Liveness** — every worker heartbeats into `indexer_workers`. A worker that
   stops heartbeating loses its shards.

Which contracts a shard contains is **derived, not stored**:

```
shard_id = fnv1a64(contract_id) % shard_count
```

`coordinator.ShardForContract` implements this, and because it is a pure
function of the contract id, every process computes the same answer without
coordination. Adding a contract never writes to the shard table, and a worker
restart never has to rebuild state.

## Roles

The same binary runs all three roles.

| Role | Command | Responsibility |
| --- | --- | --- |
| `all` | `indexer` (default) | Current behaviour: index every contract in one process. |
| `coordinator` | `indexer --role coordinator --worker-id coordinator-0` | Elect a leader, create shard rows, assign and reassign them. |
| `worker` | `indexer --role worker --worker-id worker-3` | Heartbeat, resolve its shards to contracts, index only those. |

`--worker-id` must be unique per process. It always is under Kubernetes if you
derive it from the pod name, e.g. `--worker-id=$(POD_NAME)`.

### Leader election

The coordinator takes a Postgres advisory lock, `pg_try_advisory_lock
(0x536F726F6C656E73)` — the ASCII bytes of "Sorolens". The lock is
**session-scoped**, so the coordinator must hold a dedicated connection for as
long as it leads; if the connection returns to a pool the lock is released and
another replica can (correctly) take over. `TryAdvisoryLock` never blocks: a
`false` result means another process is already leading, which is a normal state
and not an error.

Only the leader writes `indexer_shards`. Workers read it. This is what keeps
assignment deterministic — two leaders cannot disagree if only one can write.

### Assignment

On every rebalance the leader:

1. Marks workers whose `heartbeat_at` is older than `--worker-stale-after`
   (default 30s) as dead, deleting their row.
2. Collects unowned shards — never assigned, or owned by a worker that just died.
3. Hands them out round-robin over the live workers, sorted by worker id so the
   assignment is reproducible.

Shards already owned by a live worker are **not** touched. A healthy worker is
never interrupted, and no rebalancing happens while the worker set is stable.

## Failure recovery

A worker crash is bounded by two intervals:

```
recovery <= worker-stale-after + rebalance-interval
```

With the defaults that is 30s + 10s = **40 seconds** before the dead worker's
shards are owned again, and the next worker pass picks those contracts up
immediately. Nothing is lost: each contract's progress is already durable in
`sync_state` / the indexer cursor, and a contract is never processed by two
workers at once because a shard only ever has one owner.

The indexer itself is idempotent per contract — event and invocation inserts are
keyed on `(tx_hash, contract_id)` — so even a reassignment that overlaps an
in-flight pass cannot duplicate rows.

## Configuration

| Flag | Env | Default | Meaning |
| --- | --- | --- | --- |
| `--role` | `INDEXER_ROLE` | `all` | `all`, `coordinator`, or `worker`. |
| `--worker-id` | `INDEXER_WORKER_ID` | — | Unique id. Required for coordinator and worker. |
| `--shard-count` | `INDEXER_SHARD_COUNT` | `64` | Total shards. Must match across all processes. |
| `--shard-store` | `INDEXER_SHARD_STORE` | — | Shard store backend, see below. |
| — | `INDEXER_HEARTBEAT_INTERVAL` | `5s` | Worker heartbeat cadence. |
| — | `INDEXER_WORKER_STALE_AFTER` | `30s` | Silence before a worker is presumed dead. |
| — | `INDEXER_REBALANCE_INTERVAL` | `10s` | Leader reconciliation cadence. |

Choose the shard count so a single shard is comfortably smaller than one
worker's throughput: reassignment moves whole shards, so a shard that takes
longer than `worker-stale-after` to index will make a recovering worker look
slow. 64 shards over 4 workers is 16 shards each.

## Schema

`apps/api/internal/db/migrations/000011_indexer_shards.up.sql` creates
`indexer_workers` and `indexer_shards` with the indexes the coordinator's hot
paths need. Nothing else is required — the leader lock is an advisory lock, not
a row.

## Topology

```
                ┌──────────────────────┐
                │  coordinator (1)     │  advisory lock -> leader
                │  EnsureShards        │  assign / reassign
                └──────────┬───────────┘
                           │ writes indexer_shards
        ┌──────────────────┼──────────────────┐
        ▼                  ▼                  ▼
  ┌───────────┐      ┌───────────┐      ┌───────────┐
  │ worker-1  │      │ worker-2  │      │ worker-3  │  heartbeat + read shards
  │ shards    │      │ shards    │      │ shards    │  index only owned contracts
  └───────────┘      └───────────┘      └───────────┘
```

Run exactly one coordinator replica, or several for failover — the advisory lock
guarantees only one leads at a time. Workers scale freely.

## Current status and the one remaining adapter

What is implemented here:

- The sharding algorithm — `ShardForContract`, `ContractsForShards`.
- Leader election, assignment, staleness detection and reassignment —
  `coordinator.Coordinator`.
- The worker loop — `coordinator.WorkerRunner`.
- The poller scope wrapper that makes a worker index only its contracts —
  `scopedStore` in `services/indexer`.
- The schema — migration `000011`.
- Tests covering assignment, reassignment after a worker goes stale, and a
  4-worker / 100-contract end-to-end assignment check —
  `coordinator_test.go`.

**What is not wired: the shared `coordinator.Store`.** The indexer module
currently depends on no database driver — `main.go` wires a `stubStore` for
exactly this reason — so `--shard-store memory` is the only backend available
today. The in-memory store keeps the shard table inside one process, which is
fine for exercising the topology on a single node but is **not safe for
replicas**: each process would hold its own table and believe it owned every
shard. Multi-replica sharding needs a `coordinator.Store` backed by Postgres,
implementing the schema above plus the contract list. That is a self-contained
adapter; adding it means taking the `pgx` dependency in the indexer module.

Until then, `--role coordinator` and `--role worker` fail fast without
`--shard-store memory` rather than silently doing the wrong thing.
