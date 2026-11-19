DROP FUNCTION IF EXISTS archive_old_events(int);
DROP FUNCTION IF EXISTS maintain_events_partitions(int);
DROP VIEW IF EXISTS events_with_archive;
DROP TABLE IF EXISTS events_archive;
