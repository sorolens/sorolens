-- 000038_search_trgm_indexes.up.sql
-- Add trigram indexes to speed up ILIKE '%term%' substring searches
-- on label and contract columns.

CREATE EXTENSION IF NOT EXISTS pg_trogm;

-- Label search: store.postgres.go::SearchLabels uses ILIKE '%term%' on labels.name
-- and labels.description. GIN trigram indexes support ILIKE with leading wildcards.
CREATE INDEX IF NOT EXISTS
    idx_labels_name_trgm
    ON labels
    USING gin (name gin_trogm_ops);

CREATE INDEX IF NOT EXISTS
    idx_labels_description_trgm
    ON labels
    USING gin (description gin_trogm_ops);

-- Contract search: store.postgres.go::SearchContracts uses ILIKE '%term%' on contracts.address
-- and contracts.name.
CREATE INDEX IF NOT EXISTS
    idx_contracts_address_trgm
    ON contracts
    USING gin (address gin_trogm_ops);

CREATE INDEX IF NOT EXISTS
    idx_contracts_name_trgm
    ON contracts
    USING gin (name gin_trgm_ops);
