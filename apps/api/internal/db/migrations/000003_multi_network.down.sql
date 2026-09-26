DROP INDEX IF EXISTS idx_monitored_contracts_network;
ALTER TABLE monitored_contracts DROP COLUMN IF EXISTS network;

DROP INDEX IF EXISTS idx_storage_network;
ALTER TABLE storage_entries DROP COLUMN IF EXISTS network;

DROP INDEX IF EXISTS idx_invocations_network_ledger;
ALTER TABLE invocations DROP COLUMN IF EXISTS network;

DROP INDEX IF EXISTS idx_events_network_ledger;
ALTER TABLE events DROP COLUMN IF EXISTS network;

ALTER TABLE contracts ALTER COLUMN network DROP DEFAULT;
