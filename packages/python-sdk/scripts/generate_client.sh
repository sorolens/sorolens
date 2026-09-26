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

# Post-process: normalise generated output to match CI environment
"$PYTHON" - "$target" << 'PYEOF'
import re, sys
from pathlib import Path

def fix(path):
    t = path.read_text()

    # 1. TypeVar cls -> typing_extensions.Self on model from_dict methods
    if 'T = TypeVar' in t and 'def from_dict' in t:
        if 'from typing_extensions import Self' not in t:
            t = re.sub(r'(from attrs import field as _attrs_field)', r'\1\nfrom typing_extensions import Self', t)
        t = re.sub(r'def from_dict\(cls: type\[T\],', r'def from_dict(cls,', t)
        t = re.sub(r'(def from_dict\(cls,.*?\)) -> T:', r'\1 -> Self:', t, flags=re.DOTALL)
        t = re.sub(r'  # noqa: PLC0415', '', t)

    # 2. *args: Any -> *args: object in __exit__/__aexit__ (client.py)
    t = re.sub(r'(def __(?:a)?exit__\(self, \*args): Any,', r'\1: object,', t)

    path.write_text(t)

# 3. Sort __all__ in models/__init__.py (case-sensitive lexicographic)
init = Path(sys.argv[1]) / "models" / "__init__.py"
if init.exists():
    text = init.read_text()
    m = re.search(r'(__all__ = \(\n)((?:    "[^"]+",\n)+)(\))', text)
    if m:
        entries = re.findall(r'"([^"]+)"', m.group(2))
        entries.sort()
        new_block = ''.join(f'    "{e}",\n' for e in entries)
        text = text[:m.start()] + '__all__ = (\n' + new_block + ')' + text[m.end():]
        init.write_text(text)

for f in Path(sys.argv[1]).rglob("*.py"):
    fix(f)
PYEOF

echo "Regenerated $target from $spec (openapi-python-client $generator_version)"
