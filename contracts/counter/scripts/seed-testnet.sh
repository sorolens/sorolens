#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${SOROLENS_DEPLOY_SECRET:-}" ]]; then
  echo "SOROLENS_DEPLOY_SECRET must be set" >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONTRACT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$CONTRACT_DIR"

if ! command -v cargo >/dev/null 2>&1; then
  echo "cargo is required" >&2
  exit 1
fi

if ! command -v soroban >/dev/null 2>&1; then
  echo "soroban CLI is required" >&2
  exit 1
fi

cargo build --target wasm32-unknown-unknown --release

WASM_PATH="$CONTRACT_DIR/target/wasm32-unknown-unknown/release/sorolens_counter.wasm"
if [[ ! -f "$WASM_PATH" ]]; then
  echo "Expected Wasm artifact at $WASM_PATH" >&2
  exit 1
fi

NETWORK="testnet"
ACCOUNT_ID="$(soroban contract deploy \
  --wasm "$WASM_PATH" \
  --network "$NETWORK" \
  --source-account "$SOROLENS_DEPLOY_SECRET" \
  --rpc-url "https://soroban-testnet.stellar.org:443" \
  --network-passphrase 'Test SDF Network ; September 2015' \
  --build-only)"

if [[ -z "$ACCOUNT_ID" ]]; then
  echo "Failed to deploy contract" >&2
  exit 1
fi

echo "Deployed contract: $ACCOUNT_ID"

soroban contract invoke \
  --id "$ACCOUNT_ID" \
  --network "$NETWORK" \
  --source-account "$SOROLENS_DEPLOY_SECRET" \
  --rpc-url "https://soroban-testnet.stellar.org:443" \
  --network-passphrase 'Test SDF Network ; September 2015' \
  -- \
  increment --by 3

soroban contract invoke \
  --id "$ACCOUNT_ID" \
  --network "$NETWORK" \
  --source-account "$SOROLENS_DEPLOY_SECRET" \
  --rpc-url "https://soroban-testnet.stellar.org:443" \
  --network-passphrase 'Test SDF Network ; September 2015' \
  -- \
  increment --by 4

soroban contract invoke \
  --id "$ACCOUNT_ID" \
  --network "$NETWORK" \
  --source-account "$SOROLENS_DEPLOY_SECRET" \
  --rpc-url "https://soroban-testnet.stellar.org:443" \
  --network-passphrase 'Test SDF Network ; September 2015' \
  -- \
  reset

soroban contract invoke \
  --id "$ACCOUNT_ID" \
  --network "$NETWORK" \
  --source-account "$SOROLENS_DEPLOY_SECRET" \
  --rpc-url "https://soroban-testnet.stellar.org:443" \
  --network-passphrase 'Test SDF Network ; September 2015' \
  -- \
  set_label --label alpha
