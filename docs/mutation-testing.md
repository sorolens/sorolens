# Mutation testing (Go)

Line coverage says code ran; mutation testing asks whether the tests would
notice a bug. Sorolens runs [go-mutesting](https://github.com/avito-tech/go-mutesting)
(the maintained avito-tech fork; the original `zimmski/go-mutesting` does not
build on Windows) on a schedule against the security-critical packages
(issue #849):

| Package | What it protects | Baseline score |
| --- | --- | --- |
| `apps/api/internal/middleware` (auth/scope/rate-limit functions) | API key auth, scope enforcement, session/proxy HMAC verification, rate limiting | **0.43** (69/159 killed) |
| `apps/api/internal/cursor` | pagination cursor parsing and encoding — feeds keyset queries in the store layer | **0.82** (18/22 killed) |
| `apps/api/internal/reportdigest` | HMAC-signed one-click unsubscribe tokens, digest scheduling | **0.58** (was 0.33 before the tests below) |

Baselines recorded 2026-11-19 with `go-mutesting /exec-timeout:20`.

## Where it runs

`.github/workflows/mutation-testing.yml` — weekly (Monday 04:00 UTC) and on
demand via workflow_dispatch. It never runs on pull requests: CI is capped
at five PR checks, so mutation testing is a scheduled job like
`integration.yml`. Each run uploads the full log (score + the surviving
mutants with their diffs) as an artifact for 30 days.

## Running it locally

```sh
cd apps/api
go install github.com/avito-tech/go-mutesting/cmd/go-mutesting@latest
go-mutesting /exec-timeout:20 ./internal/cursor
# Limit the middleware run to the security-relevant functions:
go-mutesting /exec-timeout:20 \
  '/match:RequiredScope|RequireScopes|extractAPIKey|lookupRequiredScope|IsAdminKey|Issue|Verify|mac|resolveIdentity|resolveSession|resolveAPIKey|resolveTrustedProxy|ProxyMAC|RateLimit|clientIP|requireRole' \
  ./internal/middleware
```

Each mutant is a one-line edit (an inverted condition, a changed constant, a
dropped return); go-mutesting re-runs the package's tests against it and
reports `PASS` (mutant killed — a test failed) or `FAIL` (mutant survived —
no test noticed the change).

## What the survivors mean

Surviving mutants are not all equal:

- **Equivalent mutants** — the mutation is semantically identical
  (e.g. removing the `if raw == ""` early return in `cursor.Encode`,
  because base64 of `""` is still `""`). They cannot be killed and do not
  indicate a testing gap. All four survivors in `internal/cursor` are of
  this kind.
- **Real gaps** — behaviour nothing asserts. The 2026-11-19 baseline run
  found 54 survivors in `internal/reportdigest`; the tests added in
  `digest_survivors_test.go` pin down the ones that matter and killed 20 of
  them, including:
  - the unsubscribe token is exactly `id + "." + HMAC-SHA256(key, id)` and
    not the bare subscription id (anyone could unsubscribe anyone),
  - unsigned mode (no signing key) uses the bare id and nothing else,
  - invalid frequencies are rejected before any listing,
  - the weekday and the daily/weekly stats window passed to the store,
  - the contract-list limit clamp (`<= 0` or `> 100` → 50),
  - send errors abort the run with the pre-failure count,
  - store errors propagate.

  The remaining survivors are mostly cosmetic (HTML template strings,
  prose in the email body) and were judged not worth pinning.

## Updating the baseline

After adding tests that kill survivors (or accepting new survivors), update
the table above and reference the PR that did it.
