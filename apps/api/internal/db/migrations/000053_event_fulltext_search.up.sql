-- Issue #798: full-text search over decoded event values.
--
-- A generated tsvector column built from the JSONB representations of
-- topic_decoded and value_decoded, so a user can paste an address, symbol or
-- amount and find matching events.  to_tsvector('simple', ...) keeps tokens
-- verbatim (no stop-words, no stemming) which is what we want for contract
-- addresses and hex strings.
--
-- The column is STORED so it is computed once at write time, not at every
-- query.  The GIN index turns the tsvector into a fast set-intersection lookup.
ALTER TABLE events
    ADD COLUMN IF NOT EXISTS event_tsv tsvector
        GENERATED ALWAYS AS (
            to_tsvector('simple',
                coalesce(topic_decoded::text, '') || ' ' ||
                coalesce(value_decoded::text, '')
            )
        ) STORED;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_events_event_tsv
    ON events USING GIN (event_tsv);
