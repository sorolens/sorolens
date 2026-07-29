## Summary
Adds full ScVal decode support to the `decode()` function in `@sorolens/xdr`, covering all XDR discriminant values (0–21). Previously, only `bool`, `u64`, and `symbol` had dedicated cases — all other types fell through to `"unknown"`.

## Related issue
Closes #[TBD]

## Changes made
- **`packages/xdr/src/index.ts`** — Extended the `decode()` switch with 14 new cases:
  - `void` (1), `sc_error` (2), `u32` (3), `i32` (4), `i64` (6), `u128` (7), `i128` (8), `bitset` (11), `string` (12), `vec` (13), `map` (14), `bytes` (15), `address` (16), `contract_instance` (17), `ledger_key_instance` (18), `nonce` (19), `time_point` (20), `duration` (21)
  - `vec` and `map` use recursive `readScVal` calls to decode nested elements
  - Wide integer types (i64, u128, i128, bitset, nonce, time_point, duration) store values as strings to avoid precision loss
- **`packages/xdr/src/index.test.ts`** — Added 28 new test cases covering all new types (including edge cases: min/max, negative/zero/positive, empty collections). Total tests: 42 (up from 8).
- **`.github/workflows/ci.yml`** — Added `pnpm test` step to the Node packages CI job so decoder tests actually run in CI (only lint was running before).

## How to test
```
cd packages/xdr
pnpm test
```
All 42 tests should pass.

## Checklist
- [x] Tests added or updated for every new or changed behavior.
- [x] `pnpm vitest run` passes locally for any TypeScript changes.
- [x] `pnpm --filter "./packages/**" lint` passes.
- [ ] CI is passing on this branch.

## PR link
https://github.com/adewolescott/sorolens/compare/main...feat/scval-decode-all-types?expand=1
