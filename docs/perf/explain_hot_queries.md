# Hot query performance evidence (issue #762)

Migration `000046_trigram_and_composite_indexes` adds one composite B-tree on the
partitioned `events` table and six GIN trigram indexes on `contracts`,
`labels_public` and `labels_workspace`. This page is the evidence for those
indexes: real `EXPLAIN (ANALYZE, BUFFERS)` output captured before and after, the
storage they cost, the write penalty they impose, and the search paths they do
not fix.

Everything below was measured, not estimated. The seed, the four queries, the
write batch and the whole measurement loop are checked in under
`docs/perf/hot_queries/`, so the numbers can be regenerated with:

```sh
# PGHOST/PGPORT/PGUSER/PGPASSWORD must point at a server that allows CREATEDB
docs/perf/hot_queries/measure.sh
```

## 1. Method

**Server.** PostgreSQL 16.15 (Homebrew, aarch64-apple-darwin) on a laptop with no
other load. `shared_buffers = 128MB`, `work_mem = 4MB`,
`effective_cache_size = 4GB`, `max_parallel_workers_per_gather = 2`, collation
`en_US.UTF-8`. A production instance with a larger shared-memory allocation will
show a smaller relative win on the sequential-scan side, because the gap between
`without` and `with` here is dominated by how many heap pages Postgres has to
touch.

**Data.** `seed.sql` loads 20 000 contracts, 150 000 public account labels,
50 000 workspace labels and 1 000 000 events spread over 61 days and 500
contracts. Roughly one contract label in a hundred and one workspace and public
label in a few thousand contain the token `usdc`, which is the search term used
for the `ILIKE` queries, so a real result set is returned rather than an empty
one. `measure.sh` then appends write batches (see section 7) before reading, so
both databases are in the same shape as a running indexer.

**The pair.** Two databases with identical rows differing only in this
migration's indexes:

| DB | role | indexes from 000046 |
| :-- | :-- | :-- |
| `sorolens_perf_with` | after | present |
| `sorolens_perf_without` | before | created by the same migrations, then dropped by name |

Dropping the indexes from a `CREATE DATABASE ... TEMPLATE` clone (rather than
building a second seed and skipping the indexes) is what guarantees the rows,
the heap layout and the `Vacuum`/FSM state are byte-for-byte the same on both
sides.

**State at read time.** 50 000 contracts, 270 002 `labels_public`, 80 000
`labels_workspace`, 1 120 000 events, `invocations` empty.

**Timing.** One warm-up run plus seven timed runs per query per database. Each
run is `EXPLAIN (ANALYZE)` and the reported number is its `Execution Time`, so
planning time is excluded. The median of the seven is the headline figure; the
raw runs are in the appendix. The plan text quoted below is a separate single
warm capture, which is why its own `Execution Time` line can sit a little below
the median.

## 2. Events feed for one contract

`ListAllEvents` in `apps/api/internal/store/events_global.go` backs both
`/events` and the per-contract event page. The keyset cursor is on
`id`, the contract filter is a `LIKE 'prefix%'`, and the result is ordered by
`id DESC`.

```sql
SELECT id, contract_id, network, ledger, ledger_closed_at, tx_hash, type,
       topic_xdr, value_xdr, topic_decoded, value_decoded,
       in_successful_call, inserted_at
FROM events
WHERE ($1 = '' OR id < $1)
  AND ($2 = '' OR contract_id LIKE $2 || '%')
  AND ($3 = '' OR type = $3)
  AND ($4 = '' OR network = $4)
  AND ($5::timestamptz IS NULL OR ledger_closed_at >= $5)
  AND ($6::timestamptz IS NULL OR ledger_closed_at <= $6)
ORDER BY id DESC
LIMIT 51;
```

**Before (147.65 ms median, 105.97 ms warm):** every partition is read
completely. 35 333 of the 36 068 buffer accesses are disk reads and 747 000 rows
per worker are thrown away by the filter.

```text
 Limit  (cost=42818.55..42824.50 rows=51 width=248) (actual time=103.085..105.860 rows=51 loops=1)
   Buffers: shared hit=735 read=35333
   ->  Gather Merge  (cost=42818.55..42829.29 rows=92 width=248) (actual time=103.084..105.852 rows=51 loops=1)
         Workers Planned: 2
         Workers Launched: 2
         ->  Sort  (cost=41818.53..41818.64 rows=46 width=248) (actual time=97.937..97.940 rows=42 loops=3)
               Sort Key: events.id DESC
               Sort Method: top-N heapsort  Memory: 51kB
               ->  Parallel Append  (cost=0.00..41817.26 rows=46 width=248) (actual time=0.192..93.651 rows=747 loops=3)
                     ->  Parallel Seq Scan on events_2026_12 events_2  (cost=0.00..21245.14 rows=23 width=248) (actual time=0.401..45.856 rows=385 loops=3)
                           Filter: (contract_id ~~ 'C0000000000000000000000000003e%'::text)
                           Rows Removed by Filter: 189331
                     ->  Parallel Seq Scan on events_2026_11 events_1  (cost=0.00..20546.12 rows=22 width=248) (actual time=0.088..71.477 rows=542 loops=2)
                           Filter: (contract_id ~~ 'C0000000000000000000000000003e%'::text)
                           Rows Removed by Filter: 274560
                     ->  Parallel Seq Scan on events_default events_3  (cost=0.00..25.77 rows=1 width=248) (actual time=0.090..0.209 rows=2 loops=1)
                           Filter: (contract_id ~~ 'C0000000000000000000000000003e%'::text)
                           Rows Removed by Filter: 647
 Planning Time: 2.493 ms
 Execution Time: 105.968 ms
```

The existing `idx_events_contract_id_ledger_closed_at` (migration 000020) cannot
serve this: it is a plain btree on `contract_id` under the database collation, so
it has no operator class for `LIKE 'x%'`, and it does not carry `id`, so the
`ORDER BY id DESC LIMIT` still costs a sort over every match.

**After (9.80 ms median, 21.03 ms in this capture):** the prefix becomes an index
range and the heap visit is bounded by the matching rows only.

```text
 Limit  (cost=24382.01..24382.14 rows=51 width=247) (actual time=20.310..20.319 rows=51 loops=1)
   Buffers: shared hit=2299
   ->  Sort  (cost=24382.01..24382.29 rows=112 width=247) (actual time=20.309..20.311 rows=51 loops=1)
         Sort Key: events.id DESC
         Sort Method: top-N heapsort  Memory: 51kB
         ->  Append  (cost=513.47..24378.28 rows=112 width=247) (actual time=0.318..8.460 rows=2240 loops=1)
               ->  Bitmap Heap Scan on events_2026_11 events_1  (cost=513.47..11946.03 rows=54 width=247) (actual time=0.317..4.740 rows=1083 loops=1)
                     Filter: (contract_id ~~ 'C0000000000000000000000000003e%'::text)
                     Heap Blocks: exact=1083
                     ->  Bitmap Index Scan on events_2026_11_contract_id_ledger_closed_at_id_idx  (cost=0.00..513.46 rows=5291 width=0) (actual time=0.200..0.200 rows=1083 loops=1)
                           Index Cond: ((contract_id ~>=~ 'C0000000000000000000000000003e'::text) AND (contract_id ~<~ 'C0000000000000000000000000003f'::text))
                           Buffers: shared hit=24
               ->  Bitmap Heap Scan on events_2026_12 events_2  (cost=535.75..12421.22 rows=56 width=247) (actual time=0.314..3.563 rows=1155 loops=1)
                     Filter: (contract_id ~~ 'C0000000000000000000000000003e%'::text)
                     Heap Blocks: exact=1155
                     ->  Bitmap Index Scan on events_2026_12_contract_id_ledger_closed_at_id_idx  (cost=0.00..535.74 rows=5519 width=0) (actual time=0.199..0.199 rows=1155 loops=1)
                           Index Cond: ((contract_id ~>=~ 'C0000000000000000000000000003e'::text) AND (contract_id ~<~ 'C0000000000000000000000000003f'::text))
                           Buffers: shared hit=30
               ->  Bitmap Heap Scan on events_default events_3  (cost=4.30..10.47 rows=2 width=248) (actual time=0.025..0.027 rows=2 loops=1)
                     Filter: (contract_id ~~ 'C0000000000000000000000000003e%'::text)
                     Heap Blocks: exact=2
                     ->  Bitmap Index Scan on events_default_contract_id_ledger_closed_at_id_idx  (cost=0.00..4.29 rows=2 width=0) (actual time=0.018..0.018 rows=2 loops=1)
                           Index Cond: ((contract_id ~>=~ 'C0000000000000000000000000003e'::text) AND (contract_id ~<~ 'C0000000000000000000000000003f'::text))
 Planning Time: 5.358 ms
 Execution Time: 21.028 ms
```

`events_2026_11_contract_id_ledger_closed_at_id_idx` and its siblings are the
per-partition children that Postgres builds for
`idx_events_contract_time_id ON ONLY events (contract_id text_pattern_ops,
ledger_closed_at DESC, id DESC)`; the parent itself reports `0 bytes` because a
partitioned index stores no data.

`text_pattern_ops` is what turns `LIKE 'prefix%'` into the `~>=~ / ~<~` range
above. It is also a constraint on the index: an index on a `text_pattern_ops`
column cannot answer ordinary range comparisons (`contract_id > 'x'`) under a
non-C collation. That is why the 000020 index is kept rather than replaced.

**Limitation.** The global feed (no `contract_id` filter) is unchanged by this
migration, and correctly so: with no filter there is nothing to seek to, and
`ORDER BY id DESC LIMIT 50` already walks the primary-key index of the newest
partition. Only the filtered feed gets the 15x.

## 3. Label search

`SearchLabels` in `apps/api/internal/store/postgres.go:47`
is the account label box: one `OR` across `label` and `value` on two tables
behind a `UNION ALL`.

```sql
SELECT label, value, workspace_id, public
FROM (SELECT label, value, '' AS workspace_id, TRUE AS public FROM labels_public
      UNION ALL
      SELECT label, value, workspace_id, FALSE AS public
      FROM labels_workspace WHERE workspace_id = $1) labels
WHERE label ILIKE '%' || $2 || '%' OR value ILIKE '%' || $2 || '%'
ORDER BY label
LIMIT 100;
```

**Before (323.34 ms median, 210.20 ms warm):** two parallel sequential scans over
350 000 rows.

```text
 Limit  (cost=9896.28..9902.81 rows=56 width=128) (actual time=207.670..210.154 rows=65 loops=1)
   Buffers: shared hit=1960 read=4509
   ->  Gather Merge  (cost=9896.28..9902.81 rows=56 width=128) (actual time=207.669..210.143 rows=65 loops=1)
         Workers Planned: 2
         Workers Launched: 2
         ->  Sort  (cost=8896.26..8896.33 rows=28 width=128) (actual time=201.609..201.611 rows=22 loops=3)
               Sort Key: labels_public.label
               Sort Method: quicksort  Memory: 28kB
               ->  Parallel Append  (cost=0.00..8895.58 rows=28 width=128) (actual time=5.834..201.438 rows=22 loops=3)
                     ->  Parallel Seq Scan on labels_public  (cost=0.00..6480.59 rows=22 width=131) (actual time=4.257..165.987 rows=15 loops=3)
                           Filter: ((label ~~* '%usdc%'::text) OR (value ~~* '%usdc%'::text))
                           Rows Removed by Filter: 89986
                     ->  Parallel Seq Scan on labels_workspace  (cost=0.00..2414.85 rows=9 width=117) (actual time=6.830..53.165 rows=10 loops=2)
                           Filter: ((workspace_id = 'workspace-seed'::text) AND ((label ~~* '%usdc%'::text) OR (value ~~* '%usdc%'::text)))
                           Rows Removed by Filter: 39990
 Planning Time: 1.066 ms
 Execution Time: 210.199 ms
```

**After (0.52 ms median):** each branch is a `BitmapOr` over two trigram indexes
and the workspace filter is applied to the 20 surviving heap pages instead of to
80 000 rows.

```text
 Limit  (cost=390.27..390.44 rows=70 width=127) (actual time=0.415..0.423 rows=65 loops=1)
   Buffers: shared hit=96
   ->  Sort  (cost=390.27..390.44 rows=70 width=127) (actual time=0.414..0.417 rows=65 loops=1)
         Sort Key: labels_public.label
         Sort Method: quicksort  Memory: 33kB
         ->  Append  (cost=68.57..388.12 rows=70 width=127) (actual time=0.050..0.227 rows=65 loops=1)
               ->  Bitmap Heap Scan on labels_public  (cost=68.57..268.19 rows=54 width=131) (actual time=0.050..0.145 rows=45 loops=1)
                     Recheck Cond: ((label ~~* '%usdc%'::text) OR (value ~~* '%usdc%'::text))
                     Heap Blocks: exact=45
                     Buffers: shared hit=59
                     ->  BitmapOr  (cost=68.57..68.57 rows=54 width=0) (actual time=0.039..0.039 rows=0 loops=1)
                           ->  Bitmap Index Scan on idx_labels_public_label_trgm  (cost=0.00..34.27 rows=27 width=0) (actual time=0.028..0.028 rows=45 loops=1)
                                 Index Cond: (label ~~* '%usdc%'::text)
                           ->  Bitmap Index Scan on idx_labels_public_value_trgm  (cost=0.00..34.27 rows=27 width=0) (actual time=0.010..0.010 rows=0 loops=1)
                                 Index Cond: (value ~~* '%usdc%'::text)
               ->  Bitmap Heap Scan on labels_workspace  (cost=60.11..119.58 rows=16 width=115) (actual time=0.031..0.079 rows=20 loops=1)
                     Recheck Cond: ((label ~~* '%usdc%'::text) OR (value ~~* '%usdc%'::text))
                     Filter: (workspace_id = 'workspace-seed'::text)
                     Heap Blocks: exact=20
                     Buffers: shared hit=34
                     ->  BitmapOr  (cost=60.11..60.11 rows=16 width=0) (actual time=0.026..0.026 rows=0 loops=1)
                           ->  Bitmap Index Scan on idx_labels_workspace_label_trgm  (cost=0.00..30.05 rows=8 width=0) (actual time=0.015..0.015 rows=20 loops=1)
                                 Index Cond: (label ~~* '%usdc%'::text)
                           ->  Bitmap Index Scan on idx_labels_workspace_value_trgm  (cost=0.00..30.05 rows=8 width=0) (actual time=0.010..0.010 rows=0 loops=1)
                                 Index Cond: (value ~~* '%usdc%'::text)
 Planning Time: 2.148 ms
 Execution Time: 0.525 ms
```

Total buffer traffic falls from 6 469 pages to 96.

## 4. Contract search

`SearchContracts` in `apps/api/internal/store/postgres.go:859`
is the command-K contract search, including the `last_activity_at` subquery it
left joins.

```sql
SELECT c.id, c.network, c.label, c.wasm_hash, c.created_at_ledger,
       c.backfill_complete_at, c.status, c.added_at, activity.last_activity_at
FROM contracts c
LEFT JOIN (
    SELECT contract_id, MAX(ledger_closed_at) AS last_activity_at
    FROM (SELECT contract_id, ledger_closed_at FROM events
          UNION ALL
          SELECT contract_id, ledger_closed_at FROM invocations) activity_rows
    GROUP BY contract_id
) activity ON activity.contract_id = c.id
WHERE c.id ILIKE '%' || $1 || '%' OR c.label ILIKE '%' || $1 || '%'
ORDER BY c.added_at DESC
LIMIT 50;
```

**Before (423.05 ms median, 582.29 ms warm):** a sequential scan over `contracts`
feeds a nested loop that re-materialises the full activity aggregate.

```text
 Limit  (cost=48100.80..48100.82 rows=10 width=162) (actual time=581.313..582.207 rows=50 loops=1)
   Buffers: shared hit=2103 read=34853
   ->  Sort  (cost=48100.80..48100.82 rows=10 width=162) (actual time=581.312..582.204 rows=50 loops=1)
         Sort Key: c.added_at DESC
         Sort Method: top-N heapsort  Memory: 31kB
         ->  Nested Loop Left Join  (cost=46331.72..48100.63 rows=10 width=162) (actual time=262.463..581.995 rows=200 loops=1)
               Join Filter: (activity.contract_id = c.id)
               Rows Removed by Join Filter: 98651
               ->  Seq Scan on contracts c  (cost=0.00..1685.74 rows=10 width=154) (actual time=0.118..250.833 rows=200 loops=1)
                     Filter: ((id ~~* '%usdc%'::text) OR (label ~~* '%usdc%'::text))
                     Rows Removed by Filter: 49800
               ->  Materialize  (cost=46331.72..46385.39 rows=200 width=65) (actual time=1.307..1.398 rows=493 loops=200)
                     ->  Subquery Scan on activity  (cost=46331.72..46384.39 rows=200 width=65) (actual time=261.433..263.129 rows=500 loops=1)
                           ->  Finalize GroupAggregate  (cost=46331.72..46382.39 rows=200 width=65) (actual time=261.433..263.096 rows=500 loops=1)
                                 Group Key: events_1.contract_id
                                 ->  Gather Merge  (cost=46331.72..46378.39 rows=400 width=65) (actual time=261.423..262.932 rows=1500 loops=1)
                                       Workers Planned: 2
                                       Workers Launched: 2
                                       ->  Sort  (cost=45331.70..45332.20 rows=200 width=65) (actual time=257.617..257.634 rows=500 loops=3)
                                             Sort Key: events_1.contract_id
                                             ->  Partial HashAggregate  (cost=45322.05..45324.05 rows=200 width=65) (actual time=253.236..253.276 rows=500 loops=3)
                                                   Group Key: events_1.contract_id
                                                   ->  Parallel Append  (cost=0.00..42992.91 rows=465828 width=65) (actual time=0.030..152.101 rows=373333 loops=3)
                                                         ->  Parallel Seq Scan on events_2026_12 events_1  (cost=0.00..20654.31 rows=236331 width=65) (actual time=0.051..51.482 rows=189716 loops=3)
                                                         ->  Parallel Seq Scan on events_2026_11 events  (cost=0.00..19973.30 rows=229130 width=65) (actual time=0.027..105.740 rows=275101 loops=2)
                                                         ->  Parallel Seq Scan on events_default events_2  (cost=0.00..24.82 rows=382 width=65) (actual time=0.003..0.098 rows=649 loops=1)
                                                         ->  Parallel Seq Scan on invocations  (cost=0.00..11.35 rows=135 width=40) (actual time=0.000..0.000 rows=0 loops=1)
 Planning Time: 3.445 ms
 Execution Time: 582.293 ms
```

**After (208.77 ms median, 214.40 ms warm):** the trigram pair replaces the
`contracts` scan (250.8 ms of the plan drops to 0.77 ms) and the planner switches
the join to a hash join, so the aggregate is no longer replayed per contract.

```text
 Limit  (cost=46492.55..46492.58 rows=10 width=161) (actual time=212.733..214.226 rows=50 loops=1)
   Buffers: shared hit=2970 read=33255
   ->  Sort  (cost=46492.55..46492.58 rows=10 width=161) (actual time=212.733..214.221 rows=50 loops=1)
         Sort Key: c.added_at DESC
         Sort Method: top-N heapsort  Memory: 36kB
         ->  Hash Right Join  (cost=46439.19..46492.39 rows=10 width=161) (actual time=210.151..214.155 rows=200 loops=1)
               Hash Cond: (events_1.contract_id = c.id)
               ->  Finalize GroupAggregate  (cost=46350.43..46401.10 rows=200 width=65) (actual time=209.310..213.174 rows=500 loops=1)
                     Group Key: events_1.contract_id
                     ->  Gather Merge  (cost=46350.43..46397.10 rows=400 width=65) (actual time=209.281..212.835 rows=1500 loops=1)
                           Workers Planned: 2
                           Workers Launched: 2
                           ->  Sort  (cost=45350.41..45350.91 rows=200 width=65) (actual time=201.452..201.479 rows=500 loops=3)
                                 Sort Key: events_1.contract_id
                                 ->  Partial HashAggregate  (cost=45340.76..45342.76 rows=200 width=65) (actual time=196.191..196.244 rows=500 loops=3)
                                       Group Key: events_1.contract_id
                                       ->  Parallel Append  (cost=0.00..43006.95 rows=466763 width=65) (actual time=0.038..119.220 rows=373333 loops=3)
                                             ->  Parallel Seq Scan on events_2026_12 events_1  (cost=0.00..20662.45 rows=237145 width=65) (actual time=0.068..50.179 rows=189716 loops=3)
                                             ->  Parallel Seq Scan on events_2026_11 events  (cost=0.00..19974.51 rows=229251 width=65) (actual time=0.030..69.303 rows=275101 loops=2)
                                             ->  Parallel Seq Scan on events_default events_2  (cost=0.00..24.82 rows=382 width=65) (actual time=0.004..0.115 rows=649 loops=1)
                                             ->  Parallel Seq Scan on invocations  (cost=0.00..11.35 rows=135 width=40) (actual time=0.001..0.001 rows=0 loops=1)
               ->  Hash  (cost=88.64..88.64 rows=10 width=153) (actual time=0.810..0.813 rows=200 loops=1)
                     Buckets: 1024  Batches: 1  Memory Usage: 35kB
                     ->  Bitmap Heap Scan on contracts c  (cost=51.57..88.64 rows=10 width=153) (actual time=0.172..0.771 rows=200 loops=1)
                           Recheck Cond: ((id ~~* '%usdc%'::text) OR (label ~~* '%usdc%'::text))
                           Heap Blocks: exact=200
                           ->  BitmapOr  (cost=51.57..51.57 rows=10 width=0) (actual time=0.130..0.132 rows=0 loops=1)
                                 ->  Bitmap Index Scan on idx_contracts_id_trgm  (cost=0.00..30.04 rows=5 width=0) (actual time=0.013..0.013 rows=0 loops=1)
                                       Index Cond: (id ~~* '%usdc%'::text)
                                 ->  Bitmap Index Scan on idx_contracts_label_trgm  (cost=0.00..21.53 rows=5 width=0) (actual time=0.117..0.117 rows=200 loops=1)
                                       Index Cond: (label ~~* '%usdc%'::text)
 Planning Time: 6.127 ms
 Execution Time: 214.402 ms
```

**Read the 2x honestly.** The trigram indexes fix the part of this query they can
see. The remaining ~209 ms belongs to `activity`, which aggregates
`MAX(ledger_closed_at)` over every event in the database to answer a question
about the 50 contracts that survive the filter. No index on this migration fixes
that, and it is the reason Q3 carries the largest run-to-run spread in the
appendix (152 ms to 1 575 ms). The fix is to correlate the aggregate to the
matched contracts with a `LATERAL` over the per-partition
`(contract_id, ledger_closed_at)` index, which is a query change and therefore
out of scope for an index-only issue. It is listed in section 9.

## 5. Summary of the read side

| Query | Store function | Without 000046 | With 000046 | Change |
| :-- | :-- | --: | --: | --: |
| Contract event feed | `ListAllEvents` | 147.65 ms | 9.80 ms | 15.1x faster |
| Account label search | `SearchLabels` | 323.34 ms | 0.52 ms | 627x faster |
| Contract search (end to end) | `SearchContracts` | 423.05 ms | 208.77 ms | 2.03x faster |
| Contract search (the indexed part) | `SearchContracts` scan node | 250.83 ms | 0.77 ms | 326x faster |

## 6. Search paths this migration deliberately leaves alone

Documented so the next reader does not assume coverage that does not exist.

- **`SearchEvents` (`tx_hash ILIKE '%q%'`,
  `apps/api/internal/store/postgres.go:915`).**
  Measured at 203.7 ms (`with`) and 196.2 ms (`without`) on the pair above, at
  1 280 000 events: a `Parallel Seq Scan` on every partition, unaffected by this
  change. A `gin (tx_hash gin_trgm_ops)` index would fix it but has to be built
  per partition with `CONCURRENTLY` (see [docs/migrations.md](../migrations.md)),
  and `tx_hash` is a 64-hex-digit value where users usually paste the whole hash
  or a long prefix, so an exact/prefix lookup is the better first move. Follow-up
  in section 9, not this PR.
- **`SearchFunctions` (`function_name ILIKE '%q%'`).** Small table, and
  `invocations` was empty at seed; not worth an index on measured evidence.
- **`monitored_contracts` name search.** No store query does a substring search
  against `monitored_contracts` at all (there is no `ILIKE` on it), and the table
  is bounded by the number of watchdog registrations, so the trigram indexes an
  earlier draft of this page proposed for it were dropped from the migration. The
  only watchdog name lookup in the plans reviewed was a sequential scan over 5 000
  rows at 7.0 ms.
- **The global events feed with no filters**, as noted in section 2.

## 7. Write cost

`write_batch.sql` inserts 20 000 events, 5 000 contracts, 20 000 public labels and
5 000 workspace labels in one pass: the volume a backfill moves, and the shape of
a batched indexer flush. The batch ran against both databases in the same round,
three rounds, and the numbers below are medians of those three. Times include
`psql` startup and the `generate_series` CPU, so they are an upper bound on index
maintenance rather than a clean measurement of it, and the ratio understates the
cost when the client is a Go insert loop.

Measured on the pair after the read benchmarks, at 1 120 000 events / 50 000
contracts / 270 002 + 80 000 labels:

| Table | Rows per batch | Without | With | Delta | Per row |
| :-- | --: | --: | --: | --: | --: |
| `events` | 20 000 | 5 177.9 ms | 5 771.0 ms | +593.1 ms | +29.7 us |
| `contracts` | 5 000 | 152.1 ms | 564.3 ms | +412.2 ms | +82.4 us |
| `labels_public` | 20 000 | 2 410.6 ms | 4 888.0 ms | +2 477.4 ms | +123.9 us |
| `labels_workspace` | 5 000 | 688.4 ms | 1 453.7 ms | +765.3 ms | +153.1 us |
| **batch total** | **50 000** | **8 116.5 ms** | **12 999.0 ms** | **+4 882.5 ms (+60.2 %)** | |

Batch totals per round: without 7 882.9 / 8 116.5 / 8 535.7 ms, with 11 629.1 /
12 999.0 / 14 989.6 ms. The `with` spread is wider than the `without` spread,
which is the expected GIN signature: inserts land in the pending-list and the
cost surfaces in a later autovacuum or `gin_pending_list_limit` flush, so the
batch that pays for it is not always the batch that caused it.

What this means for the service:

- **The indexer is the only heavy writer and the events table is the cheapest one
  here:** +593 ms on a 5 178 ms insert, about 11 % on the biggest volume in the
  system. The composite index is safe to keep permanently.
- **The label tables are read-heavy and written rarely** (a backfill and manual
  label imports), so 124-153 us per row is a cost paid in batches that run
  occasionally. Keep.
- **If a bulk relabel has to run against production,** drop
  `idx_labels_public_value_trgm` first: it is the largest index in the migration
  (section 8) and, of the six, the least likely to be what a user is searching
  for, since `value` holds a Stellar address that the existing
  `idx_labels_public_value` btree already matches exactly.
- Nothing in this migration is on a per-request write path.

## 8. Storage cost

`pg_relation_size` on the `with` database at 1 280 000 events, 90 000 contracts,
430 002 public labels, 120 000 workspace labels (the row counts grew while section
7 ran, so this is a slightly larger database than section 1's read state):

| Index | Table | Size | Notes |
| :-- | :-- | --: | :-- |
| `idx_events_contract_time_id` | `events` (children) | 214.6 MiB | 175.8 B/row, vs 99.7 B/row for the existing 000020 pair |
| `idx_labels_public_value_trgm` | `labels_public` | 120.7 MiB | largest add in the migration |
| `idx_labels_public_label_trgm` | `labels_public` | 58.8 MiB | |
| `idx_labels_workspace_value_trgm` | `labels_workspace` | 19.7 MiB | |
| `idx_labels_workspace_label_trgm` | `labels_workspace` | 16.4 MiB | |
| `idx_contracts_id_trgm` | `contracts` | 15.7 MiB | |
| `idx_contracts_label_trgm` | `contracts` | 7.8 MiB | |
| **total** | | **453.8 MiB** | 78 % of it is the events index and the two `value` trigram indexes |

The parent `idx_events_contract_time_id` reports 0 bytes; the size is in its
per-partition children, and it grows with the event table like any other index on
it. Retention or partition drops reclaim it with the partition.

## 9. Follow-ups this evidence justifies

1. Correlate `SearchContracts`' activity aggregate to the matched contracts
   (`LATERAL` + the per-partition `(contract_id, ledger_closed_at)` index) to take
   the residual ~209 ms out of the command-K path.
2. Give `SearchEvents` a prefix/exact strategy on `tx_hash`, or a per-partition
   trigram index built with `CONCURRENTLY` if substring search is a product
   requirement. Today it scans every partition on every keystroke.
3. Revisit `idx_labels_public_value_trgm` once there is data on how often users
   search a substring of an address rather than pasting one. It is 120.7 MiB and
   the largest write cost in the migration.

## 10. Connection pooling

Independent of the indexes, and the reason a fast plan can still look slow at the
endpoint:

1. **PgBouncer**
   - `pool_mode = transaction` for the API's HTTP handlers.
   - Pool size 20-30 per API replica, reserve 5 for bursts, cap clients around
     5 000.
   - `transaction` mode needs `max_prepared_statements` (PgBouncer 1.21+) or the
     equivalent in another pooler: pgx v5 caches prepared statements per
     connection, and in transaction mode a session's statements are re-bound to
     different server connections, so an uncached prepared statement fails or
     leaks. The alternative is `pgxpool.WithStatementCache` off / `prepare_threshold
     = -1`, which costs a parse round trip per query.
2. **Serverless Postgres (Neon, Supabase, Neon-style branches)**
   - Pooled endpoint (`-pooler.*`) for `apps/api` request handlers.
   - Direct, unpooled connection for `golang-migrate` and for the indexer's
     streaming listeners, both of which hold state a transaction pooler cannot
     preserve (`LISTEN`/`NOTIFY` and advisory locks are the ones that break
     silently).

## 11. How this is kept true

- `TestTrigramAndCompositeIndexesMigration` in
  `apps/api/internal/store/indexes_perf_test.go`
  asserts the migration creates each named index, that the `down` migration drops
  the same set, and, through `TestTrigramIndexTargetsExist`, that every table the
  migration indexes is a table some migration actually creates. That last guard
  is what would have caught the original draft of this page, which indexed an
  `account_labels` table that does not exist in the schema.
- `idx_events_contract_time_id` is created without `CONCURRENTLY` and is listed in
  `apps/api/internal/db/migration_lint_allowlist.txt` because PostgreSQL rejects
  `CONCURRENTLY` on a partitioned parent; [docs/migrations.md](../migrations.md)
  carries the per-partition recipe for deployments that cannot tolerate the lock.
- Migrations were applied and rolled back on an empty database and against the
  seeded database in this PR's gate run.

## Appendix: raw execution times

`EXPLAIN (ANALYZE)` `Execution Time` in ms, one warm-up discarded.

| Query | Database | runs |
| :-- | :-- | :-- |
| Q1 events feed | with | 11.207, 9.804, 9.526, 21.410, 9.513, 9.763, 11.991 |
| Q1 events feed | without | 147.650, 203.466, 158.232, 140.042, 127.837, 108.497, 171.704 |
| Q2 label search | with | 0.609, 0.455, 0.434, 0.516, 0.951, 0.891, 0.435 |
| Q2 label search | without | 323.338, 277.735, 482.192, 313.562, 314.255, 720.271, 767.434 |
| Q3 contract search | with | 161.765, 152.105, 163.158, 208.774, 373.935, 1574.651, 880.435 |
| Q3 contract search | without | 423.051, 290.103, 427.489, 419.099, 437.610, 350.650, 434.408 |
