#!/bin/sh
# Start the bot. When LITESTREAM_BUCKET is set, first restore the link
# database from the replica (if one exists), then run the bot under
# Litestream so every change is streamed back to the bucket.
set -eu

if [ -z "${LITESTREAM_BUCKET:-}" ]; then
  echo "[entrypoint] LITESTREAM_BUCKET not set: links are NOT backed up and are lost on redeploy" >&2
  exec node dist/index.js
fi

: "${LITESTREAM_PATH:=sorolens-discord-bot}"
: "${LITESTREAM_REGION:=auto}"
: "${LITESTREAM_ENDPOINT:=}"
export LITESTREAM_PATH LITESTREAM_REGION LITESTREAM_ENDPOINT

echo "[entrypoint] restoring $DATABASE_PATH from replica (if one exists)"
litestream restore -config /etc/litestream.yml \
  -if-db-not-exists -if-replica-exists "$DATABASE_PATH"

exec litestream replicate -config /etc/litestream.yml -exec "node dist/index.js"
