#!/usr/bin/env bash
# Regenerate the low-level client under src/sorolens/_generated from the OpenAPI spec.
#
# The generated layer is checked in verbatim; do not edit it by hand. This script is
# what CI (`python-sdk` workflow, "generated client up-to-date" job) runs to make sure
# the checked-in code still matches docs/openapi.yaml.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo_root="$(cd "$here/../.." && pwd)"
spec="${1:-$repo_root/docs/openapi.yaml}"
target="$here/src/sorolens/_generated"
generator_version="0.29.1"

if [[ ! -f "$spec" ]]; then
  echo "OpenAPI spec not found at $spec" >&2
  exit 1
fi

PYTHON="${PYTHON:-python3}"
if ! command -v "$PYTHON" >/dev/null 2>&1; then
  PYTHON=python
fi
if ! command -v "$PYTHON" >/dev/null 2>&1; then
  echo "No python interpreter found (set PYTHON=...)" >&2
  exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

"$PYTHON" -m pip install --quiet "openapi-python-client==$generator_version"
"$PYTHON" -m openapi_python_client generate --path "$spec" --meta none --output-path "$tmp/client"

rm -rf "$target"
cp -R "$tmp/client" "$target"

echo "Regenerated $target from $spec (openapi-python-client $generator_version)"
