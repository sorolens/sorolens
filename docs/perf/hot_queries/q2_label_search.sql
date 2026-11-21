-- apps/api/internal/store/postgres.go, SearchLabels: the account label search
-- box. One OR over two columns of two tables behind a UNION ALL.
EXPLAIN (ANALYZE, BUFFERS)
SELECT label, value, workspace_id, public
FROM (SELECT label, value, '' AS workspace_id, TRUE AS public FROM labels_public
      UNION ALL
      SELECT label, value, workspace_id, FALSE AS public
      FROM labels_workspace WHERE workspace_id = 'workspace-seed') labels
WHERE label ILIKE '%' || 'usdc' || '%' OR value ILIKE '%' || 'usdc' || '%'
ORDER BY label
LIMIT 100;
