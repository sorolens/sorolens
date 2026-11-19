# @sorolens/xdr

Dependency-free decoding of Soroban `ScVal` XDR (base64) into readable values.

## `decode(base64)`

Returns `{ type, value, human }` for every `ScVal` type:

| `type`                                       | `value`                                        | `human` example                                       |
| -------------------------------------------- | ---------------------------------------------- | ----------------------------------------------------- |
| `bool`                                       | `boolean`                                      | `true`                                                |
| `void`                                       | `null`                                         | `void`                                                |
| `u32`, `i32`                                 | `number`                                       | `42`                                                  |
| `u64`, `i64`, `u128`, `i128`, `u256`, `i256` | decimal `string` (survives JSON)               | `-1000000000000000000000`                             |
| `timepoint`                                  | seconds as `string`                            | `2026-01-01T00:00:00.000Z`                            |
| `duration`                                   | seconds as `string`                            | `3600s`                                               |
| `bytes`                                      | `0x…` hex `string`                             | `0xdeadbeef`                                          |
| `string`                                     | `string`                                       | `"hello"` (quoted)                                    |
| `symbol`                                     | `string`                                       | `transfer`                                            |
| `address`                                    | StrKey `string` (`G…`, `C…`, `M…`, `B…`, `L…`) | `GAAA…WHF`                                            |
| `error`                                      | `{ type, code }`                               | `Error(Contract, #7)`, `Error(Budget, ExceededLimit)` |
| `vec`                                        | `DecodedScVal[]`                               | `[1, 2]`                                              |
| `map`                                        | `{ key, value }[]` (keys stay typed)           | `{amount: 5}`                                         |
| `contract_instance`                          | `{ executable, storage }`                      | `ContractInstance(wasm:…, {…})`                       |
| `ledger_key_contract_instance`               | `null`                                         | `LedgerKeyContractInstance`                           |
| `ledger_key_nonce`                           | `string`                                       | `Nonce(99)`                                           |

Malformed or truncated input never throws; it returns
`{ type: "error", value: null, human: "<decode_error>" }`.

`decodeScVal(base64)` keeps its original loose output (plain JS values) for
existing callers, and `decodeTopic(topics)` maps it over event topics.

## `decodeWithSpec(base64, spec)`

Names the parts of a value using a contract spec type, so the output reads
like the contract's own Rust types:

```ts
import { decodeWithSpec, type SpecType } from "@sorolens/xdr";

const point: SpecType = {
  kind: "struct",
  name: "Point",
  fields: [
    { name: "x", type: { kind: "primitive" } },
    { name: "y", type: { kind: "primitive" } },
  ],
};
const action: SpecType = {
  kind: "union",
  name: "Action",
  cases: [{ name: "Stop" }, { name: "Move", types: [point] }],
};

decodeWithSpec(xdr, action).human; // "Action::Move(Point { x: 1, y: 2 })"
```

Supported spec kinds: `struct` (named fields from a map, or tuple fields from a
vec), `union` (`[Symbol(case), ...payload]`), `enum` (`u32` → variant name),
`vec`, `map`, `option`, `tuple` and `primitive`. Matched values carry
`specName` and, for unions and enums, `variant`.

Where a value does not match its spec (wrong shape, unknown variant, missing
field), that part is returned exactly as `decode` returns it, so a partial or
outdated spec never breaks decoding.
