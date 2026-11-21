-- apps/api/internal/store/events_global.go, ListAllEvents, with a contract
-- prefix filter (the dashboard's "events of one contract" page).
-- Run with: psql -v ON_ERROR_STOP=1 -v prefix="<contract id prefix>" -f this
EXPLAIN (ANALYZE, BUFFERS)
SELECT id, contract_id, network, ledger, ledger_closed_at, tx_hash, type,
       topic_xdr, value_xdr, topic_decoded, value_decoded,
       in_successful_call, inserted_at
FROM events
WHERE ('' = '' OR id < '')
  AND (:'prefix' = '' OR contract_id LIKE :'prefix' || '%')
  AND ('' = '' OR type = '')
  AND ('' = '' OR network = '')
  AND (NULL::timestamptz IS NULL OR ledger_closed_at >= NULL)
  AND (NULL::timestamptz IS NULL OR ledger_closed_at <= NULL)
ORDER BY id DESC
LIMIT 51;
