-- apps/api/internal/store/postgres.go, SearchContracts: the command-K contract
-- search, including the last-activity subquery it joins.
EXPLAIN (ANALYZE, BUFFERS)
SELECT c.id, c.network, c.label, c.wasm_hash, c.created_at_ledger,
       c.backfill_complete_at, c.status, c.added_at, activity.last_activity_at
FROM contracts c
LEFT JOIN (
    SELECT contract_id, MAX(ledger_closed_at) AS last_activity_at
    FROM (
        SELECT contract_id, ledger_closed_at FROM events
        UNION ALL
        SELECT contract_id, ledger_closed_at FROM invocations
    ) activity_rows
    GROUP BY contract_id
) activity ON activity.contract_id = c.id
WHERE c.id ILIKE '%' || 'usdc' || '%' OR c.label ILIKE '%' || 'usdc' || '%'
ORDER BY c.added_at DESC
LIMIT 50;
