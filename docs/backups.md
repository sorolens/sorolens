# Backups and Point-In-Time-Recovery (PITR)

This document describes the backup and restore procedures for the SoroLens PostgreSQL database.

## Backup Scripts

The `scripts/backup.sh` script creates a logical backup (`pg_dump`) and a physical base backup (`pg_basebackup`). Both are uploaded to S3.
WAL (Write-Ahead Logging) archiving is enabled in `docker-compose.yml`, which automatically pushes WAL segments to S3, ensuring Point-In-Time-Recovery is possible.

### Running a Backup

\`\`\`bash
./scripts/backup.sh
\`\`\`
This requires AWS CLI configured to access the `sorolens-backups` S3 bucket.

## Restore Procedure (PITR)

The `scripts/restore.sh` script downloads a base backup from S3, extracts it, and configures `postgresql.auto.conf` and `recovery.signal` for Point-In-Time-Recovery.

### Running a Restore

1. Run the restore script, specifying the base backup and target time.
   \`\`\`bash
   ./scripts/restore.sh s3://sorolens-backups/base/base_20231001_120000.tar.gz '2023-10-01 13:00:00'
   \`\`\`
2. The script will prepare the data directory in `./postgres_data_restore`.
3. Stop your running Postgres container and mount this new directory as the data volume.
   \`\`\`bash
   # For docker-compose, you can update the volume mount temporarily, or copy the contents into your existing volume mount.
   \`\`\`
4. Start Postgres. It will replay WAL from S3 until it reaches the target time and then promote itself to a primary.
