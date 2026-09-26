#!/usr/bin/env bash
set -e

if [ "$#" -lt 2 ]; then
    echo "Usage: $0 <s3-base-backup-path> <target-time>"
    echo "Example: $0 s3://sorolens-backups/base/base_20260923_120000.tar.gz '2026-09-23 13:00:00'"
    exit 1
fi

S3_BASE=$1
TARGET_TIME=$2
S3_BUCKET="${S3_BUCKET:-s3://sorolens-backups}"
DATA_DIR="./postgres_data_restore"

echo "Downloading base backup from S3..."
aws s3 cp "$S3_BASE" base_restore.tar.gz

echo "Extracting base backup..."
mkdir -p "$DATA_DIR"
tar -xzf base_restore.tar.gz -C "$DATA_DIR"

echo "Configuring recovery for PITR..."
touch "$DATA_DIR/recovery.signal"
cat <<EOF >> "$DATA_DIR/postgresql.auto.conf"
restore_command = 'aws s3 cp $S3_BUCKET/wal/%f %p'
recovery_target_time = '$TARGET_TIME'
recovery_target_action = 'promote'
EOF

echo "Restore configured in $DATA_DIR."
echo "Start Postgres using this data directory."
rm base_restore.tar.gz
