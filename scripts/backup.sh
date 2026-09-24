#!/usr/bin/env bash
set -e

S3_BUCKET="${S3_BUCKET:-s3://sorolens-backups}"
DB_CONTAINER="${DB_CONTAINER:-sorolens-postgres-1}"
DB_USER="${DB_USER:-sorolens}"
DB_NAME="${DB_NAME:-sorolens}"
DATE=$(date +%Y%m%d_%H%M%S)

# 1. Logical Backup (pg_dump)
echo "Taking pg_dump logical backup..."
DUMP_FILE="backup_${DATE}.sql.gz"
docker exec -t "$DB_CONTAINER" pg_dump -U "$DB_USER" "$DB_NAME" | gzip > "$DUMP_FILE"

echo "Uploading logical backup to S3..."
aws s3 cp "$DUMP_FILE" "$S3_BUCKET/logical/$DUMP_FILE"
rm "$DUMP_FILE"

# 2. Physical Base Backup for PITR
echo "Taking pg_basebackup physical backup..."
BASE_BACKUP_DIR="basebackup_${DATE}"
docker exec -t "$DB_CONTAINER" pg_basebackup -U "$DB_USER" -D "/tmp/$BASE_BACKUP_DIR" -Ft -z -P
docker exec "$DB_CONTAINER" sh -c "cat /tmp/$BASE_BACKUP_DIR/base.tar.gz" > "base_${DATE}.tar.gz"
docker exec -t "$DB_CONTAINER" rm -rf "/tmp/$BASE_BACKUP_DIR"

echo "Uploading base backup to S3..."
aws s3 cp "base_${DATE}.tar.gz" "$S3_BUCKET/base/base_${DATE}.tar.gz"
rm "base_${DATE}.tar.gz"

echo "Backup completed successfully."
