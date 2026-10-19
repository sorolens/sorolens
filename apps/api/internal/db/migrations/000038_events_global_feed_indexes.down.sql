-- Roll back the global events feed composite indexes from 000038.
DROP INDEX IF EXISTS events_network_closed_at_id_idx;
DROP INDEX IF EXISTS events_type_closed_at_id_idx;
DROP INDEX IF EXISTS events_closed_at_id_idx;
DROP INDEX IF EXISTS events_contract_closed_at_id_idx;
