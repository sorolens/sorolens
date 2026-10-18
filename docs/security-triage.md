# Security Triage

This document describes the security scanning pipeline and how to triage findings.

## Scans

The `Security Scans` workflow (`.github/workflows/security.yml`) runs on every pull request targeting `main`, on pushes to `main`, and weekly on Monday at 06:00 UTC. It can also be triggered manually via `workflow_dispatch`.

| Job | Tool | Scope | Fails on |
| --- | --- | --- | --- |
| `codeql` | CodeQL | Go and TypeScript source code | High / critical findings (via branch protection + code scanning requirements) |
| `govulncheck` | govulncheck | Go modules and call graph | Any reported vulnerability in used code |
| `cargo-audit` | cargo audit | Rust crates in `contracts/` | Advisories with severity >= warned |
| `pip-audit` | pip-audit | Python dependencies in `requirements.txt` | Any reported vulnerability not explicitly ignored |

All findings are uploaded to the repository's GitHub Security tab (Code scanning and Dependency alerts) where applicable.

## Triage workflow

1. **Identify the finding.** Open the failed job log or the Security tab and note the advisory ID, affected package, version, and severity.
2. **Assess exposure.** Determine whether the vulnerable code path is actually reachable in our application. `govulncheck` only reports reachable vulnerabilities, but `pip-audit` and `cargo audit` report on declared dependencies regardless of reachability.
3. **Remediate or document.** Prefer upgrading the dependency. If an upgrade is not possible immediately, add a documented exception (below).

## Documented exceptions

Exceptions must be minimal, justified, and reviewed on a schedule. Each exception MUST include:

- The advisory ID (e.g. `GH-SA-2024-0001`, `RSEC-2024-0001`)
- The affected package and version range
- A rationale explaining why the vulnerability is not exploitable in our context
- An owner and a review date (no more than 90 days out)

### Python (`pip-audit`)

Add an entry to `.pip-audit-ignore.txt`, one entry per line:

```
GHS-SA-2024-0001 # reason: vulnerable function not called; owner: @maintainer; review: 2025-01-01
```

### Rust (`cargo audit`)

Add an entry to `.cargo/audit.toml`:

```toml
[advisories]
ignore = [
  "RSEC-2024-0001", # reason: not reachable; owner: @maintainers; review: 2025-01-01
]
```

### Go (`govulncheck`)

Prefer upgrading the module. If an upgrade is impossible, document the exception in this file with the same fields as above and reference it in the PR description.

### CodeQL

CodeQL findings can be dismissed in the GitHub Security tab with a documented reason. Dismissals are audited by GitHub and must include a justification and an owner.

## Escalation

If a finding is judged to be exploitable in production, follow the process in `SECURITY.md` (or contact the maintainers privately via Discord) and do not open a public issue with exploit details.
