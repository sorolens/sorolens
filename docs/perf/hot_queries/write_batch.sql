-- One write batch for the index-maintenance measurement in measure.sh:
-- 20 000 events, 5 000 contracts, 20 000 public labels and 5 000 workspace
-- labels, using ledger ids and md5 salts derived from :base so repeated
-- batches never collide.
WITH ids AS (SELECT array_agg(id ORDER BY id) a FROM (SELECT id FROM contracts ORDER BY id LIMIT 500) s)
INSERT INTO events (id, contract_id, network, ledger, ledger_closed_at, tx_hash, type, topic_xdr, value_xdr, topic_decoded)
SELECT lpad((:base + g)::text, 19, '0') || '-0000000001',
       ids.a[1 + (g % 500)], 'public', :base + g,
       TIMESTAMPTZ '2026-11-01 00:00:00+00' + ((g % 61) || ' days')::interval + ((g % 86400) || ' seconds')::interval,
       md5(('e' || :base || g)::text), 'transfer', '["AAAAAgAAAAA="]'::jsonb, 'vdr' || g, '["transfer"]'::jsonb
FROM generate_series(1, 20000) g, ids;

INSERT INTO contracts (id, network, label, status, added_at)
SELECT 'C' || substr(md5(('c' || :base)::text || g::text) || md5(('c2' || :base)::text || g::text) || md5(('c3' || :base)::text || g::text), 1, 55),
       'public', 'contract ' || g, 'active', now()
FROM generate_series(1, 5000) g;

INSERT INTO labels_public (label, value)
SELECT 'acct ' || md5(('l' || :base)::text || g::text),
       'G' || substr(md5(('v' || :base)::text || g::text) || md5(('v2' || :base)::text || g::text) || md5(('v3' || :base)::text || g::text), 1, 55)
FROM generate_series(1, 20000) g;

INSERT INTO labels_workspace (workspace_id, label, value)
SELECT 'workspace-seed', 'ws ' || md5(('w' || :base)::text || g::text),
       'G' || substr(md5(('x' || :base)::text || g::text) || md5(('x2' || :base)::text || g::text) || md5(('x3' || :base)::text || g::text), 1, 55)
FROM generate_series(1, 5000) g;
