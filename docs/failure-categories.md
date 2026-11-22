# Failure categories

A failure rate tells you that invocations fail, not why. When the indexer
stores a `FAILED` invocation it also sorts the failure into one of a small set
of categories, and keeps the raw error code next to it, so you can see where to
look (issue #806).

The category and code are exposed on every invocation in the API
(`failure_category`, `failure_code`), the invocation lists filter by
`failure_category`, and `GET /api/v1/contracts/{id}/invocations/stats` counts
failures per category in each window (`failures_by_category`). See the
[API reference](api/reference.md).

## Categories

| Category          | Meaning                                                                               | Host error type (`ScErrorType`)                                        | `InvokeHostFunction` result code                                                                                           |
| ----------------- | ------------------------------------------------------------------------------------- | ---------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| `budget_exceeded` | The invocation ran out of the CPU, memory, ledger I/O or refundable fee it declared.  | `Budget`                                                               | `RESOURCE_LIMIT_EXCEEDED`, `INSUFFICIENT_REFUNDABLE_FEE`                                                                   |
| `contract_error`  | The contract returned one of its own error codes (a `#[contracterror]` value).        | `Contract`                                                             | `TRAPPED`, with the error in the diagnostic events                                                                         |
| `auth_failure`    | Authorization failed: a missing or invalid signature, or an unauthorized call.        | `Auth`                                                                 | `TRAPPED`, with the error in the diagnostic events                                                                         |
| `storage`         | A ledger entry was missing, archived, or could not be read or written.                | `Storage`                                                              | `ENTRY_ARCHIVED`                                                                                                           |
| `wasm_trap`       | The contract's Wasm trapped, for example on a panic, an `unreachable`, or a bad call. | `WasmVm`                                                               | `TRAPPED`, with the error in the diagnostic events                                                                         |
| `other`           | Anything else. The raw code is always kept, so nothing is dropped.                    | `Context`, `Object`, `Crypto`, `Events`, `Value`, and any unknown type | `MALFORMED`, `TRAPPED` without diagnostic events, any unknown code; transaction-level failures such as `txSOROBAN_INVALID` |

Successful invocations, and invocations indexed before classification existed,
have no category: the fields are omitted from API responses and `NULL` in the
database.

## How a failure is classified

The indexer reads two sources returned by `getTransaction`:

1. **The transaction result** (`resultXdr`). For an `InvokeHostFunction`
   operation its result code says whether the host ran out of resources, met
   an archived entry, or trapped. Fee-bump transactions are unwrapped to their
   inner result.
2. **The diagnostic events** (`diagnosticEventsXdr`). The Soroban host logs
   every error as an event with the topics `["error", Error(<type>, <code>)]`.
   The second topic is an `ScError`: an `ScErrorType` plus either the
   contract's own code (type `Contract`) or an `ScErrorCode` such as
   `ExceededLimit`, `MissingValue` or `InvalidAction`.

The rules apply in this order:

1. A result code that names the cause on its own wins:
   `RESOURCE_LIMIT_EXCEEDED` and `INSUFFICIENT_REFUNDABLE_FEE` are
   `budget_exceeded`, and `ENTRY_ARCHIVED` is `storage`.
2. Otherwise the **last** `error` diagnostic event decides, by its
   `ScErrorType` (table above). The last one is the error that ended the
   invocation; an earlier one may have been caught by a `try_call`.
3. Otherwise the failure is `other`. In particular, `TRAPPED` on its own does
   not say why the host trapped: an RPC node only returns diagnostic events
   when it has them enabled, and without them a trap stays `other`, with
   `failure_code` set to `INVOKE_HOST_FUNCTION_TRAPPED`.

Classification never fails and never drops an invocation. Anything unknown or
undecodable is filed under `other` with the raw value kept.

## The raw code

`failure_code` holds the error behind the category, in one of these forms:

| Form                          | Example                                               | Source                                                                                                                   |
| ----------------------------- | ----------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| `Error(<Type>, <Code>)`       | `Error(Budget, ExceededLimit)`, `Error(Contract, #4)` | Host error from the diagnostic events, printed as the host and `stellar-cli` print it. A contract code is shown as `#n`. |
| `INVOKE_HOST_FUNCTION_<NAME>` | `INVOKE_HOST_FUNCTION_TRAPPED`                        | Operation result code, when no host error was reported.                                                                  |
| `tx<NAME>`                    | `txSOROBAN_INVALID`                                   | Transaction result code, when no operation ran.                                                                          |
| `UNKNOWN`                     | `UNKNOWN`                                             | Nothing could be decoded. The raw `TransactionResult` is still stored in `result_xdr`.                                   |

A type or code value that is not in the current XDR is kept as a number, for
example `Error(#12, #3)`, `INVOKE_HOST_FUNCTION(#-9)` or `tx(#-42)`. When the
result code decides the category but a host error was also reported (for
example `RESOURCE_LIMIT_EXCEEDED` with `Error(Budget, ExceededLimit)`), the more
specific host error is stored.

## Querying

```bash
# Every contract-error failure across tracked contracts, newest first
curl "$API/api/v1/invocations?failure_category=contract_error"

# One contract's failures of one category
curl "$API/api/v1/contracts/$CONTRACT/invocations?failure_category=budget_exceeded"

# Per-category counts for the last 24 hours, 7 days and 30 days
curl "$API/api/v1/contracts/$CONTRACT/invocations/stats"
```

A value outside the list returns `422` with code `INVALID_INPUT`, like the
other filters. `failures_by_category` always lists every category, with `0`
when a category has no failures. Failed invocations indexed before
classification existed count toward `failed_invocations` but toward no
category, so the categories can add up to less than `failed_invocations`.

## Code

- The category list:
  [`apps/api/failurecategory`](https://github.com/sorolens/sorolens/blob/main/apps/api/failurecategory/failurecategory.go),
  shared by the indexer and the API.
- The classifier:
  [`services/indexer/internal/failure`](https://github.com/sorolens/sorolens/blob/main/services/indexer/internal/failure/failure.go).
  `ScError` values are decoded by
  [`services/indexer/internal/scval`](https://github.com/sorolens/sorolens/blob/main/services/indexer/internal/scval/scerror.go).
