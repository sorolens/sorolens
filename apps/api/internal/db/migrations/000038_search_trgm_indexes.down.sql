-- Reverse the search trigram indexes added in 000038_search_trgm_indexes.up.sql
-- Drop indexes before the extension so the down migration is safe even if the extension is shared.
DROP INDEX IF EXISTS idx_contracts_label_trgm;
DROP INDEX IF EXISTS idx_contracts_name_trgm;
DROP INDEX IF EXISTS idx_contracts_address_trgm;

-- Drop the pg_trgm extension only if nothing else depends on it.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_extension e
        JOIN pg_dependency d ON (d.refobjid = e.oid AND d.deptyp = 'e')
        WHERE e.extname = 'pg_trgm'
    ) THEN
        EXECUTE USING IF EXISTS 'DROP EXTENSION IF EXISTS pg_trgm';
    END IF;
END $$;