#!/usr/bin/env bash
# Reproduce the issue #762 EXPLAIN measurements.
#
# Creates two seeded databases that differ only in the indexes this migration
# adds, runs each hot query seven times and prints the median, then times the
# same write batch against both. Takes roughly 15 minutes and needs about
# 3 GB of disk per database.
#
# Connection parameters come from the usual libpq environment
# (PGHOST/PGPORT/PGUSER/PGPASSWORD), and the target server must allow CREATEDB.
set -euo pipefail

cd "$(dirname "$0")/../.."
MIGRATIONS=apps/api/internal/db/migrations
HERE=docs/perf/hot_queries
WITH=sorolens_perf_with
WITHOUT=sorolens_perf_without

NEW_INDEXES="idx_contracts_id_trgm idx_contracts_label_trgm
idx_labels_public_label_trgm idx_labels_public_value_trgm
idx_labels_workspace_label_trgm idx_labels_workspace_value_trgm
idx_events_contract_time_id"

apply_migrations() {
  for f in $(ls "$MIGRATIONS"/*.up.sql | sort); do
    psql -q -v ON_ERROR_STOP=1 -d "$1" -f "$f" > /dev/null
  done
}

echo "== creating $WITH and seeding it"
psql -q -v ON_ERROR_STOP=1 -d postgres -c "DROP DATABASE IF EXISTS $WITH" \
  -c "CREATE DATABASE $WITH"
apply_migrations "$WITH"
psql -q -v ON_ERROR_STOP=1 -d "$WITH" -f "$HERE/seed.sql" > /dev/null

echo "== cloning the seed into $WITHOUT and dropping the #762 indexes"
psql -q -v ON_ERROR_STOP=1 -d postgres -c "DROP DATABASE IF EXISTS $WITHOUT" \
  -c "CREATE DATABASE $WITHOUT TEMPLATE $WITH"
for i in $NEW_INDEXES; do
  psql -q -v ON_ERROR_STOP=1 -d "$WITHOUT" -c "DROP INDEX IF EXISTS $i"
done
for db in "$WITH" "$WITHOUT"; do
  psql -q -d "$db" -c "ANALYZE contracts; ANALYZE labels_public; ANALYZE labels_workspace; ANALYZE events"
done

PREFIX=$(psql -q -At -d "$WITH" -c "SELECT left(id, 30) FROM contracts ORDER BY id LIMIT 1")
echo "== contract prefix used for the feed query: $PREFIX"

# The write batch runs first in each database so both end up with the same row
# counts; it doubles as the write-overhead measurement.
echo "== write overhead: five batches per database, in seconds"
for db in "$WITH" "$WITHOUT"; do
  printf '%s ' "$db"
  for r in 0 1 2 3 4; do
    /usr/bin/time -p psql -q -v ON_ERROR_STOP=1 -d "$db" \
      -v base=$((900000000 + r * 1000000)) -f "$HERE/write_batch.sql" 2>&1 \
      | awk '/^real/ {printf "%.1f ", $2}'
  done
  echo
done

echo "== read latency: warm-up plus seven timed runs, median in ms"
for q in q1_events_feed q2_label_search q3_contract_search; do
  for db in "$WITHOUT" "$WITH"; do
    printf '%-18s %-24s ' "$q" "$db"
    for i in 0 1 2 3 4 5 6 7; do
      psql -q -At -v ON_ERROR_STOP=1 -d "$db" -v prefix="$PREFIX" -f "$HERE/$q.sql" \
        | sed -n 's/.*Execution Time: \([0-9.]*\) ms.*/\1/p' >> /tmp/sorolens_perf_times.txt
    done
    tail -7 /tmp/sorolens_perf_times.txt | sort -n | awk '{a[NR]=$1} END {printf "%.2f\n", a[int((NR+1)/2)]}'
  done
done

echo
echo "Plan text for each query (paste over the captured output in the document):"
for q in q1_events_feed q2_label_search q3_contract_search; do
  for db in "$WITHOUT" "$WITH"; do
    echo "--- $q on $db"
    psql -q -v ON_ERROR_STOP=1 -d "$db" -v prefix="$PREFIX" -f "$HERE/$q.sql"
  done
done

echo "== done; drop them with: psql -d postgres -c 'DROP DATABASE $WITH' -c 'DROP DATABASE $WITHOUT'"
