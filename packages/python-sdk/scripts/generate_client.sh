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

# Post-process: convert TypeVar cls pattern to typing_extensions.Self (matches CI output)
"$PYTHON" - "$target" << 'PYEOF'
import re, sys
from pathlib import Path

def fix(path):
    t = path.read_text()
    if 'T = TypeVar' not in t or 'def from_dict' not in t:
        return
    if 'from typing_extensions import Self' not in t:
        t = re.sub(r'(from attrs import field as _attrs_field)', r'\1\nfrom typing_extensions import Self', t)
    t = re.sub(r'def from_dict\(cls: type\[T\],', r'def from_dict(cls,', t)
    t = re.sub(r'(def from_dict\(cls,.*?\)) -> T:', r'\1 -> Self:', t, flags=re.DOTALL)
    t = re.sub(r'  # noqa: PLC0415', '', t)
    path.write_text(t)

for f in Path(sys.argv[1]).rglob("*.py"):
    fix(f)
PYEOF

echo "Regenerated $target from $spec (openapi-python-client $generator_version)"
