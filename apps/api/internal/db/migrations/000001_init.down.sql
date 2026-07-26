-- Reverse of 000001_init.up.sql
-- Tables are dropped in reverse dependency order so foreign key constraints
-- are satisfied before the referenced table is removed.

DROP TABLE IF EXISTS sync_state;
DROP TABLE IF EXISTS storage_entries;
DROP TABLE IF EXISTS invocations;
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS contracts;
