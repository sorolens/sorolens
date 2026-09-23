-- ============================================================
-- 000003_multi_network
--
-- Makes the schema network-aware. The indexer and API can track and
-- query contracts on testnet, mainnet, and futurenet simultaneously.
--
-- Backward compatibility: every existing row is attributed to
-- 'testnet' (the only network available in v0.1/v0.2), and every new
-- `network` column defaults to 'testnet' so older writers that do not
-- yet set the column keep working.
-- ============================================================

-- contracts.network already exists from 000001_init. Give it a default
-- and backfill any NULL/empty values so it can be treated as NOT NULL.
UPDATE contracts SET network = 'testnet' WHERE network IS NULL OR network = '';
ALTER TABLE contracts ALTER COLUMN network SET DEFAULT 'testnet';

-- ------------------------------------------------------------
-- events
-- ------------------------------------------------------------
ALTER TABLE events ADD COLUMN network TEXT NOT NULL DEFAULT 'testnet';
UPDATE events e
   SET network = c.network
  FROM contracts c
 WHERE c.id = e.contract_id
   AND e.network IS DISTINCT FROM c.network;

CREATE INDEX idx_events_network_ledger ON events (network, ledger DESC);

-- ------------------------------------------------------------
-- invocations
-- ------------------------------------------------------------
ALTER TABLE invocations ADD COLUMN network TEXT NOT NULL DEFAULT 'testnet';
UPDATE invocations i
   SET network = c.network
  FROM contracts c
 WHERE c.id = i.contract_id
   AND i.network IS DISTINCT FROM c.network;

CREATE INDEX idx_invocations_network_ledger ON invocations (network, ledger DESC);

-- ------------------------------------------------------------
-- storage_entries
-- ------------------------------------------------------------
ALTER TABLE storage_entries ADD COLUMN network TEXT NOT NULL DEFAULT 'testnet';
UPDATE storage_entries s
   SET network = c.network
  FROM contracts c
 WHERE c.id = s.contract_id
   AND s.network IS DISTINCT FROM c.network;

CREATE INDEX idx_storage_network ON storage_entries (network);

-- ------------------------------------------------------------
-- monitored_contracts
-- The watchdog dashboard shares the header network selector, so its
-- contracts carry a network too. Defaults to testnet for existing rows.
-- ------------------------------------------------------------
ALTER TABLE monitored_contracts ADD COLUMN network TEXT NOT NULL DEFAULT 'testnet';
CREATE INDEX idx_monitored_contracts_network ON monitored_contracts (network);
