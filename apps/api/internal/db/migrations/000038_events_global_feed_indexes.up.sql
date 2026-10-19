-- Composite indexes for the global events feed (issue #97)
// The global feed query in apps/api/internal/store/events_global.go filters on
CONTRACT_ID LIKE $prefix || '%', type, network, and a ledger_closed_at range,
then orders by id DESC with a keyset cursor. These indexes are shaped to
that access pattern. Write cost is kept in mind: only the columns the
feed actually filters on are indexed, and no index includes the large
topic/value blobs.

-- Primary feed index: contract prefix + time range, ordered by id DESC.
-- text_pattern_ops is required for LIKE 'prefix%' to use the index under
-- the default (non-C) collation.
-- This covers the common "filter by contract + date range" feed view.
CREATE INDEX IF NOT EXISTS events_contract_closed_at_id_idx
	ON events (contract_id text_pattern_ops, ledger_closed_at DESC, id DESC);

-- Time-range feed index: cross-contract time slicing with keyset paging.
-- Supports the "Since/Until only" and "type + Since/Until" feed views.
CREATE INDEX IF NOT EXISTS events_closed_at_id_idx
	ON events (ledger_closed_at DESC, id DESC);

-- Type + time range feed index: the feed often filters by event type and a
-- date range together. Leading with type keeps the index selective when a
-- specific type is requested.
CREATE INDEX IF NOT EXISTS events_type_closed_at_id_idx
	ON events (type, ledger_closed_at DESC, id DESC);

-- Network + time range feed index: the feed also filters by network.
-- Network cardinality is low (4 values), but combining it with the time
-- range keeps the scan on the network's slice of the feed.
CREATE INDEX IF NOT EXISTS events_network_closed_at_id_idx
	ON events (network, ledger_closed_at DESC, id DESC);
