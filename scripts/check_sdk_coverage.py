#!/usr/bin/env python3
"""Verify every SDK exposes every operation declared in ``docs/openapi.yaml``.

This is the guard described in issue #556: the three hand-maintained SDKs
(``packages/rust-sdk``, the ``packages/python-sdk`` facade and
``packages/go-client``) each expose only a slice of the REST surface, and
nothing previously failed when a new OpenAPI operation was added without a
matching SDK method.

How coverage is decided
------------------------
For every ``operationId`` in the spec we look for a correspondingly named
symbol in the SDK's *public surface*:

* **Rust** — the ``endpoints! { fn <name>(...) } `` method generators in
  ``packages/rust-sdk/src``.
* **Python** — the operations imported by the ``sorolens.Client`` facade from
  the generated layer (``client.py``).
* **Go** — the exported ``func (c *Client{,WithResponses}) <Name>`` methods of
  the generated client.

Operation ids are camelCase (``listContractEvents``) while the SDKs use their
own casing (snake_case / PascalCase). We compare them after *canonicalising*
both sides: lowercase and strip everything that is not ``[a-z0-9]``. That makes
``listContractEvents``, ``list_contract_events`` and ``ListContractEvents`` all
collapse to ``listcontractevents``. The spec's operation ids are chosen so this
projection is collision-free.

Any operation an SDK does not (yet) expose is expected to be listed in
``scripts/sdk_coverage_allowlist.json`` as an intentional exclusion, together
with a reason. The check fails when

* an operation is missing from an SDK and is **not** allowlisted (a new
  operation landed without SDK support or an explicit exemption), or
* an allowlisted operation has since become covered (the exemption is stale and
  should be removed so the guard stays meaningful).

Run it with ``python3 scripts/check_sdk_coverage.py`` (no third-party
dependencies). It is wired into CI as the ``sdk-coverage`` job.
"""

from __future__ import annotations

import fnmatch
import json
import re
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
SPEC = REPO_ROOT / "docs" / "openapi.yaml"
ALLOWLIST = REPO_ROOT / "scripts" / "sdk_coverage_allowlist.json"

OPERATION_ID_RE = re.compile(r"^\s*operationId:\s*['\"]?([A-Za-z0-9_]+)['\"]?\s*$")
IMPORT_BLOCK_RE = re.compile(
    r"from \._generated\.api\.[a-zA-Z0-9_]+ import \(([^)]*)\)"
)
IMPORT_INLINE_RE = re.compile(
    r"from \._generated\.api\.[a-zA-Z0-9_]+ import ([^\n(]+)"
)
PY_ALIAS_RE = re.compile(r"[a-zA-Z0-9_]+(?=\s+as\s+[a-zA-Z0-9_]+)")
ENDPOINTS_BLOCK_RE = re.compile(r"endpoints!\s*\{(.*?)\n\}", re.S)
RUST_FN_RE = re.compile(r"\bfn\s+([a-z0-9_]+)\s*\(")
GO_METHOD_RE = re.compile(
    r"^func \(c \*(?:Client|ClientWithResponses)\) ([A-Za-z0-9_]+)\(", re.M
)


def canonical(name: str) -> str:
    """Lowercase and strip non-alphanumerics so casing conventions collapse."""
    return re.sub(r"[^a-z0-9]", "", name.lower())


def load_operations() -> list[str]:
    """Ordered, de-duplicated operation ids declared in the spec."""
    ops: list[str] = []
    seen: set[str] = set()
    with SPEC.open(encoding="utf-8") as fh:
        for line in fh:
            m = OPERATION_ID_RE.match(line)
            if m and m.group(1) not in seen:
                seen.add(m.group(1))
                ops.append(m.group(1))
    if not ops:
        sys.exit(f"ERROR: no operationId found in {SPEC}")
    return ops


def rust_symbols() -> set[str]:
    names: set[str] = set()
    for path in sorted((REPO_ROOT / "packages" / "rust-sdk" / "src").glob("*.rs")):
        text = path.read_text(encoding="utf-8")
        for block in ENDPOINTS_BLOCK_RE.findall(text):
            for fn in RUST_FN_RE.findall(block):
                names.add(canonical(fn))
    return names


def python_symbols() -> set[str]:
    names: set[str] = set()
    client_py = (
        REPO_ROOT
        / "packages"
        / "python-sdk"
        / "src"
        / "sorolens"
        / "client.py"
    )
    text = client_py.read_text(encoding="utf-8")
    for group in IMPORT_BLOCK_RE.findall(text):
        for token in group.split(","):
            token = token.strip()
            if not token:
                continue
            # `foo as bar` -> we care about the imported symbol `foo`.
            m = PY_ALIAS_RE.match(token)
            name = m.group(0) if m else token.split()[0]
            names.add(canonical(name))
    for line in IMPORT_INLINE_RE.findall(text):
        for token in line.split(","):
            token = token.strip()
            if not token:
                continue
            m = PY_ALIAS_RE.match(token)
            name = m.group(0) if m else token.split()[0]
            names.add(canonical(name))
    return names


def go_symbols() -> set[str]:
    names: set[str] = set()
    client_dir = REPO_ROOT / "packages" / "go-client" / "internal" / "client"
    for path in sorted(client_dir.glob("*.go")):
        if path.name.endswith("_test.go"):
            continue
        for method in GO_METHOD_RE.findall(path.read_text(encoding="utf-8")):
            names.add(canonical(method))
    return names


SDKS = {
    "rust": rust_symbols,
    "python": python_symbols,
    "go": go_symbols,
}


def load_allowlist() -> dict[str, list[tuple[str, str]]]:
    """Return ``{sdk: [(glob, reason), ...]}`` of intentional exclusions.

    The allowlist is grouped into sections per SDK; every section pairs a
    human-readable ``reason`` with the operations (or ``*``/``?`` globs) it
    covers. Exact operation ids and globs are both valid here — matching uses
    ``fnmatch`` either way.
    """
    if not ALLOWLIST.exists():
        sys.exit(f"ERROR: allowlist not found at {ALLOWLIST}")
    data = json.loads(ALLOWLIST.read_text(encoding="utf-8"))
    exclusions = data.get("exclusions", {})
    result: dict[str, list[tuple[str, str]]] = {sdk: [] for sdk in SDKS}
    for sdk, sections in exclusions.items():
        if sdk not in result:
            sys.exit(f"ERROR: allowlist references unknown SDK '{sdk}'")
        for section in sections:
            reason = section.get("reason", "").strip()
            if not reason:
                sys.exit(f"ERROR: allowlist section for '{sdk}' has no reason")
            entries = list(section.get("operations", [])) + list(
                section.get("patterns", [])
            )
            if not entries:
                sys.exit(f"ERROR: allowlist section '{reason}' lists no operations")
            for entry in entries:
                result[sdk].append((entry, reason))
    return result


def main() -> int:
    operations = load_operations()
    allow = load_allowlist()

    failures = 0
    print(f"Checking {len(operations)} OpenAPI operations across {len(SDKS)} SDKs.\n")

    for sdk, collect in SDKS.items():
        symbols = collect()
        sdk_entries = allow.get(sdk, [])

        missing: list[str] = []
        unexpected: list[str] = []
        stale: list[str] = []
        for op in operations:
            covered = canonical(op) in symbols
            allowlisted = any(
                fnmatch.fnmatchcase(op.lower(), glob.lower())
                for glob, _reason in sdk_entries
            )
            if not covered and not allowlisted:
                unexpected.append(op)
            if not covered:
                missing.append(op)
            if covered and allowlisted:
                stale.append(op)

        status = "OK"
        if unexpected or stale:
            status = "FAIL"
            failures += 1

        covered = len(operations) - len(missing)
        print(
            f"[{status}] {sdk}: {covered}/{len(operations)} operations exposed"
            + (f", {len(missing)} allowlisted" if missing else "")
        )
        for op in sorted(unexpected):
            print(f"    MISSING (not exposed, not allowlisted): {op}")
        for op in sorted(stale):
            print(
                f"    STALE ALLOWLIST (now exposed, remove the exemption): {op}"
            )
        print()

    if failures:
        print(
            "SDK coverage check failed. Either expose the operation in the SDK "
            "or document it as an intentional exclusion in "
            "scripts/sdk_coverage_allowlist.json."
        )
        return 1
    print("SDK coverage check passed.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
