#!/usr/bin/env bash
#
# backfill-callgraph.sh - materialise the cross-contract call graph for
# transactions that were indexed before call-graph tracing existed.
#
# The indexer records call_edges as it indexes new ledgers. Transactions
# already in the `invocations` table have no edges, so this script selects the
# ones the call graph is missing, asks a Soroban RPC node for each transaction's
# diagnostic events, and applies the resulting INSERTs.
#
# Usage:
#   DATABASE_URL=postgres://... SOROBAN_RPC_URL=https://... \
#     ./scripts/backfill-callgraph.sh
#
# Environment:
#   DATABASE_URL        required - postgres connection string
#   SOROBAN_RPC_URL     required - Soroban RPC endpoint that exposes
#                                 diagnostic events (ENABLE_SOROBAN_DIAGNOSTIC_EVENTS=true)
#   SOROBAN_NETWORK     optional - testnet | mainnet | futurenet (default: testnet)
#   BACKFILL_LIMIT      optional - max tx hashes per run (default: unlimited)
#   BACKFILL_BIN        optional - path to a prebuilt backfill-callgraph binary
#                                 (default: built into a temp dir with `go build`)
#
# The script is idempotent: call_edges is keyed by (tx_hash, child_span_id) and
# every INSERT uses ON CONFLICT DO NOTHING, so re-running is safe.
#
# Note: a transaction whose RPC node does not expose diagnostic events (or that
# makes no cross-contract calls) yields no edges and stays in the selection on
# the next run. That is expected on public RPC endpoints; point this at a node
# with diagnostic events enabled. Set BACKFILL_LIMIT to process in batches.
set -euo pipefail

: "${DATABASE_URL:?DATABASE_URL is required}"
: "${SOROBAN_RPC_URL:?SOROBAN_RPC_URL is required}"
SOROBAN_NETWORK="${SOROBAN_NETWORK:-testnet}"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Resolve the backfill binary. When BACKFILL_BIN is not supplied, build the
# command on the fly so the script has no build-time prerequisite.
WORKDIR=""
cleanup() {
  if [ -n "$WORKDIR" ]; then
    rm -rf "$WORKDIR"
  fi
}
trap cleanup EXIT

if [ -n "${BACKFILL_BIN:-}" ]; then
  BIN="$BACKFILL_BIN"
else
  WORKDIR="$(mktemp -d)"
  BIN="$WORKDIR/backfill-callgraph"
  echo "Building backfill-callgraph..." >&2
  (cd "$REPO_ROOT/services/indexer" && go build -o "$BIN" ./cmd/backfill-callgraph)
fi

# Select transactions that have no call graph yet, oldest first. The result is
# piped as one hash per line into the backfill command (psql -At gives a bare,
# unaligned hash per line).
HASH_QUERY="
SELECT i.tx_hash
FROM invocations i
LEFT JOIN call_edges e ON e.tx_hash = i.tx_hash
WHERE e.tx_hash IS NULL
ORDER BY i.ledger ASC, i.tx_hash ASC"

if [ -n "${BACKFILL_LIMIT:-}" ]; then
  HASH_QUERY="$HASH_QUERY
LIMIT ${BACKFILL_LIMIT}"
fi

echo "Backfilling call graph on network '$SOROBAN_NETWORK'..." >&2

psql "$DATABASE_URL" -Atc "$HASH_QUERY" \
  | "$BIN" -rpc-url "$SOROBAN_RPC_URL" -network "$SOROBAN_NETWORK" \
  | psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -q

echo "Call graph backfill complete." >&2
