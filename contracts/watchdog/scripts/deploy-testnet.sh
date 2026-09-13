#!/usr/bin/env bash
set -euo pipefail

# Build and deploy the Sorolens Watchdog contract to Stellar testnet.
#
# Required env vars:
#   SOROLENS_DEPLOY_SECRET   Stellar secret key or configured soroban CLI alias.
# Optional env vars:
#   WATCHDOG_ADMIN           Address to install as admin (defaults to deployer).
#
# On success the contract id is written to `contracts/watchdog/.deploy/testnet.txt`.

if [[ -z "${SOROLENS_DEPLOY_SECRET:-}" ]]; then
  echo "SOROLENS_DEPLOY_SECRET must be set" >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONTRACT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$CONTRACT_DIR"

for cmd in cargo soroban; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "$cmd is required" >&2
    exit 1
  fi
done

cargo build --target wasm32v1-none --release

WASM_PATH="$CONTRACT_DIR/target/wasm32v1-none/release/sorolens_watchdog.wasm"
if [[ ! -f "$WASM_PATH" ]]; then
  echo "Expected Wasm artifact at $WASM_PATH" >&2
  exit 1
fi

NETWORK="testnet"
RPC_URL="https://soroban-testnet.stellar.org:443"
PASSPHRASE='Test SDF Network ; September 2015'

CONTRACT_ID="$(soroban contract deploy \
  --wasm "$WASM_PATH" \
  --network "$NETWORK" \
  --source-account "$SOROLENS_DEPLOY_SECRET" \
  --rpc-url "$RPC_URL" \
  --network-passphrase "$PASSPHRASE")"

if [[ -z "$CONTRACT_ID" ]]; then
  echo "Failed to deploy contract" >&2
  exit 1
fi

echo "Deployed watchdog contract: $CONTRACT_ID"

ADMIN="${WATCHDOG_ADMIN:-$(soroban keys address "$SOROLENS_DEPLOY_SECRET" 2>/dev/null || echo "$SOROLENS_DEPLOY_SECRET")}"

echo "Initialising with admin: $ADMIN"

soroban contract invoke \
  --id "$CONTRACT_ID" \
  --network "$NETWORK" \
  --source-account "$SOROLENS_DEPLOY_SECRET" \
  --rpc-url "$RPC_URL" \
  --network-passphrase "$PASSPHRASE" \
  -- \
  initialize --admin "$ADMIN"

mkdir -p "$CONTRACT_DIR/.deploy"
echo "$CONTRACT_ID" > "$CONTRACT_DIR/.deploy/testnet.txt"

echo ""
echo "Next steps:"
echo "  1. Add WATCHDOG_CONTRACT_ID=$CONTRACT_ID to your .env"
echo "  2. Restart the indexer to pick up watchdog events"
echo "  3. Register a contract:"
echo "       soroban contract invoke --id $CONTRACT_ID --network testnet \\"
echo "         --source-account \$SOROLENS_DEPLOY_SECRET -- register_contract \\"
echo "         --caller \$ADDR --contract_id \$MONITORED --name my_app --check_interval 300"
