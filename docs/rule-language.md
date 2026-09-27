# The Sorolens rule language

Sorolens ships a small expression language so a team can encode its own SLOs
without a code change:

```
fee_per_invocation > 0.5 XLM for 5m on network testnet
```

A rule is stored with `POST /api/v1/rules`, validated on save, and evaluated by
the indexer at the end of every pass against per-minute metric buckets.

## Grammar

```
rule        := comparison clause*
clause      := "for" duration
             | "on" "contract" (ident | string)
             | "on" "network" ident
comparison  := term op term
op          := ">" | ">=" | "<" | "<=" | "==" | "!="
term        := number unit? | aggregation | metric | "(" comparison ")"
aggregation := func "(" metric ")"
func        := "avg" | "max" | "min" | "sum" | "rate" | "count"
metric      := ident
duration    := number ("s" | "m" | "h" | "d" | "w")
unit        := "XLM" | "stroops" | "instructions" | "bytes" | "ledgers" | "%"
```

- Numbers may be integers, decimals, or scientific notation (`5e6`).
- `5%` is shorthand for `0.05`; a bare ratio (`0.05`) means the same thing.
- A unit word is optional and only checked for compatibility with the metric on
  the other side of the comparison; it exists for readability.
- Clauses may appear in any order after the comparison.

## Semantics

**Aggregations** reduce the metric over the evaluation window into one number,
and the comparison is applied once:

- `avg(m)` / `max(m)` / `min(m)` – mean / maximum / minimum across buckets.
- `sum(m)` – total across buckets.
- `rate(m)` – total divided by the window length in seconds (per-second rate).
- `count(m)` – number of buckets that carried a value.

**A bare metric with a `for` window is a sustained condition:** every bucket in
the window must satisfy the comparison. This is how the acceptance example
reads — `fee_per_invocation > 0.5 XLM for 5m` fires only when the average cost
was above the threshold for a full five minutes.

**Without a `for` clause** the verdict rests on the most recent bucket only, so
`error_rate > 0.5` is an instantaneous check.

**Missing data never fires a rule.** If the metric was not observed in the
window, the result is "no verdict" with a reason, not a firing.

## Metrics

| Metric | Unit | Meaning |
|---|---|---|
| `invocations` | count | Transactions invoking the contract. |
| `events` | count | Events emitted. |
| `failed_invocations` | count | Invocations whose transaction failed. |
| `error_rate` | ratio | Failed invocations ÷ total. |
| `uptime` | ratio | Fraction of health checks reporting Healthy. |
| `fee_per_invocation` | XLM | Average fee per invocation. |
| `fee_per_invocation_stroops` | stroops | Same, in stroops. |
| `total_fee` | XLM | Total fee charged in the window. |
| `cpu_insn_per_invocation` | instructions | Average CPU instructions per invocation. |
| `cpu_insn_total` | instructions | Total CPU instructions. |
| `mem_byte_per_invocation` | bytes | Average memory bytes per invocation. |
| `ledger_read_bytes` / `ledger_write_bytes` | bytes | Ledger I/O in the window. |
| `storage_entries` | count | Live storage entries. |
| `expiring_storage_entries` | count | Entries expiring within seven days. |
| `min_storage_ttl_ledgers` | ledgers | Ledgers until the soonest entry expires. |
| `health_score` | score | Composite 0-100 health score. |

## Validation

Rules are validated before storage, so a malformed rule can never reach the
evaluator. Errors carry a line, a column, and — where possible — a hint:

```
1:1: unknown metric "eror_rate" (did you mean error_rate?)
```

`POST /api/v1/rules/validate` returns the same diagnostics without writing, and
`POST /api/v1/rules/preview` runs the real evaluator against a contract's recent
samples so the dashboard editor can show the verdict live.

## API

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/v1/rules` | List stored rules. |
| `POST` | `/api/v1/rules` | Create a rule (validated). |
| `PATCH` | `/api/v1/rules/{id}` | Enable or pause a rule. |
| `DELETE` | `/api/v1/rules/{id}` | Delete a rule. |
| `POST` | `/api/v1/rules/validate` | Validate rule text. |
| `POST` | `/api/v1/rules/preview` | Evaluate against live samples. |
| `GET` | `/api/v1/rules/metrics` | Metric catalog. |
| `GET` | `/api/v1/rules/library` | Curated sample rules. |

## Where it runs

The grammar, validator, and evaluator live in `apps/api/rulelang`, a leaf
package with no dependencies. The API uses it to validate and store rules; the
indexer imports the same package through the Go workspace, and its
`internal/rulesengine` evaluates enabled rules at the end of every poll pass
and raises an alert for each firing rule.

## Library

`rulelang.Library()` ships a set of validated starter rules, surfaced at
`GET /api/v1/rules/library` and in the dashboard editor. See
`apps/api/rulelang/library.go`.
