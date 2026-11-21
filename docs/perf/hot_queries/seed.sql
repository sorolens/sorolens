-- Seed used for the issue #762 EXPLAIN measurements.
-- Run against a database that already has every migration applied:
--
--   for f in $(ls apps/api/internal/db/migrations/*.up.sql | sort); do
--     psql -v ON_ERROR_STOP=1 -d "$DB" -f "$f"
--   done
--   psql -v ON_ERROR_STOP=1 -d "$DB" -f docs/perf/hot_queries/seed.sql
--
-- Row counts after the seed: 20 000 contracts, 150 000 public labels,
-- 50 000 workspace labels and 1 000 000 events across November, December 2026
-- and the default partition. The values are synthetic; the point is that the
-- searched columns are selective (about one row in 3 300 matches '%usdc%') and
-- that the tables are big enough for the planner to prefer an index scan.
BEGIN;

INSERT INTO users (id, github_id, role) VALUES ('workspace-seed', '0', 'admin');

-- 20 000 contracts; every 100th label contains the search token 'usdc'.
INSERT INTO contracts (id, network, label, status, added_at)
SELECT 'C' || lpad(md5(g::text), 55, '0'),
       'public',
       CASE
           WHEN g % 100 = 0 THEN 'usdc bridge vault ' || g
           ELSE 'contract ' || (g % 500) || ' ' || md5((g * 7)::text)
       END,
       'active',
       TIMESTAMPTZ '2026-11-01 00:00:00+00' - (g || ' minutes')::interval
FROM generate_series(1, 20000) g;

-- 150 000 public account labels; 45 of them match '%usdc%'.
INSERT INTO labels_public (label, value)
SELECT CASE
           WHEN g % 3333 = 0 THEN 'usdc treasury ' || g
           ELSE 'acct ' || md5((g * 3)::text) || ' ' || g
       END,
       'G' || substr(md5(g::text) || md5((g + 1)::text) || md5((g + 2)::text), 1, 55)
FROM generate_series(1, 150000) g;

-- 50 000 workspace labels; 20 of them match '%usdc%'.
INSERT INTO labels_workspace (workspace_id, label, value)
SELECT 'workspace-seed',
       CASE
           WHEN g % 2500 = 0 THEN 'usdc signer ' || g
           ELSE 'ws acct ' || md5((g * 5)::text) || ' ' || g
       END,
       'G' || substr(md5((g + 7)::text) || md5((g + 8)::text) || md5((g + 9)::text), 1, 55)
FROM generate_series(1, 50000) g;

-- 1 000 000 events over 61 days, spread across the 500 lowest contract ids.
WITH ids AS (
    SELECT array_agg(id ORDER BY id) a
    FROM (SELECT id FROM contracts ORDER BY id LIMIT 500) s
)
INSERT INTO events (id, contract_id, network, ledger, ledger_closed_at, tx_hash, type,
                    topic_xdr, value_xdr, topic_decoded)
SELECT lpad((1000000 + g)::text, 19, '0') || '-' || lpad((g % 100)::text, 10, '0'),
       ids.a[1 + (g % 500)],
       'public',
       1000000 + g,
       TIMESTAMPTZ '2026-11-01 00:00:00+00'
           + ((g % 61) || ' days')::interval
           + ((g % 86400) || ' seconds')::interval,
       md5(g::text),
       'transfer',
       '["AAAAAgAAAAA="]'::jsonb,
       'value-xdr-' || g,
       '["transfer"]'::jsonb
FROM generate_series(1, 1000000) g, ids;

COMMIT;

ANALYZE contracts;
ANALYZE labels_public;
ANALYZE labels_workspace;
ANALYZE events;
