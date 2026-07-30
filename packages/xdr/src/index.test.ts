import { describe, it, expect } from "vitest";
import { decode, decodeScVal } from "./index";

// ScVal bool: i32 discriminant 0 (SCV_BOOL) followed by the 4-byte XDR bool.
const BOOL_TRUE = "AAAAAAAAAAE=";
const BOOL_FALSE = "AAAAAAAAAAA=";

describe("decode - bool", () => {
  it("decodes bool true", () => {
    expect(decode(BOOL_TRUE)).toEqual({
      type: "bool",
      value: true,
      human: "true",
    });
  });

  it("decodes bool false", () => {
    expect(decode(BOOL_FALSE)).toEqual({
      type: "bool",
      value: false,
      human: "false",
    });
  });

  it("returns an error result for invalid base64", () => {
    expect(decode("not base64!!")).toEqual({
      type: "error",
      value: null,
      human: "<decode_error>",
    });
  });
});

// ScVal symbol: i32 discriminant 10 (SCV_SYMBOL) followed by a length-prefixed,
// 4-byte-padded string.
const SYMBOL_TRANSFER = "AAAACgAAAAhUcmFuc2Zlcg==";
const SYMBOL_MINT = "AAAACgAAAARtaW50";
const SYMBOL_EMPTY = "AAAACgAAAAA=";

describe("decode - symbol", () => {
  it("decodes a symbol", () => {
    expect(decode(SYMBOL_TRANSFER)).toEqual({
      type: "symbol",
      value: "Transfer",
      human: "Transfer",
    });
  });

  it("decodes a symbol whose length needs padding", () => {
    expect(decode(SYMBOL_MINT)).toEqual({
      type: "symbol",
      value: "mint",
      human: "mint",
    });
  });

  it("decodes an empty symbol without error", () => {
    expect(decode(SYMBOL_EMPTY)).toEqual({
      type: "symbol",
      value: "",
      human: "",
    });
  });
});

// ScVal u32: i32 discriminant 3 (SCV_U32) followed by a 4-byte big-endian u32.
const U32_ZERO = "AAAAAwAAAAA=";
const U32_MAX = "AAAAA/////8=";

describe("decode - u32", () => {
  it("decodes u32 value 0 correctly", () => {
    expect(decode(U32_ZERO)).toEqual({
      type: "u32",
      value: 0,
      human: "0",
    });
  });

  it("decodes u32 max value 4294967295 correctly", () => {
    expect(decode(U32_MAX)).toEqual({
      type: "u32",
      value: 4294967295,
      human: "4294967295",
    });
  });
});

// ScVal i32: i32 discriminant 4 (SCV_I32) followed by a 4-byte big-endian i32.
const I32_NEG_ONE = "AAAABP////8=";
const I32_ZERO = "AAAABAAAAAA=";
const I32_POSITIVE = "AAAABAAAACo=";

describe("decode - i32", () => {
  it("decodes i32 value -1 correctly", () => {
    expect(decode(I32_NEG_ONE)).toEqual({
      type: "i32",
      value: -1,
      human: "-1",
    });
  });

  it("decodes i32 value 0 correctly", () => {
    expect(decode(I32_ZERO)).toEqual({
      type: "i32",
      value: 0,
      human: "0",
    });
  });

  it("decodes i32 value 42 correctly", () => {
    expect(decode(I32_POSITIVE)).toEqual({
      type: "i32",
      value: 42,
      human: "42",
    });
  });
});

// ScVal i64: i32 discriminant 6 (SCV_I64) followed by an 8-byte big-endian i64.
const I64_NEG_ONE = "AAAABv//////////";
const I64_ZERO = "AAAABgAAAAAAAAAA";
const I64_POSITIVE = "AAAABgAAAAAAAAAq";

describe("decode - i64", () => {
  it("decodes i64 value -1 correctly", () => {
    expect(decode(I64_NEG_ONE)).toEqual({
      type: "i64",
      value: "-1",
      human: "-1",
    });
  });

  it("decodes i64 value 0 correctly", () => {
    expect(decode(I64_ZERO)).toEqual({
      type: "i64",
      value: "0",
      human: "0",
    });
  });

  it("decodes i64 value 42 correctly", () => {
    expect(decode(I64_POSITIVE)).toEqual({
      type: "i64",
      value: "42",
      human: "42",
    });
  });
});

// ScVal i128: i32 discriminant 8 (SCV_I128) followed by a 16-byte big-endian i128.
const I128_NEG_ONE = "AAAACP////////////////////8=";
const I128_ZERO = "AAAACAAAAAAAAAAAAAAAAAAAAAA=";
const I128_POSITIVE = "AAAACAAAAAAAAAAAAAAAAAAAACo=";

describe("decode - i128", () => {
  it("decodes i128 value -1 correctly", () => {
    expect(decode(I128_NEG_ONE)).toEqual({
      type: "i128",
      value: "-1",
      human: "-1",
    });
  });

  it("decodes i128 value 0 correctly", () => {
    expect(decode(I128_ZERO)).toEqual({
      type: "i128",
      value: "0",
      human: "0",
    });
  });

  it("decodes i128 value 42 correctly", () => {
    expect(decode(I128_POSITIVE)).toEqual({
      type: "i128",
      value: "42",
      human: "42",
    });
  });
});

// ScVal void: i32 discriminant 1 (SCV_VOID).
const VOID = "AAAAAQ==";

describe("decode - void", () => {
  it("decodes void", () => {
    expect(decode(VOID)).toEqual({
      type: "void",
      value: null,
      human: "void",
    });
  });
});

// ScVal error: i32 discriminant 2 (SCV_ERROR) followed by a 4-byte error code.
const ERROR_CODE = "AAAAAgAAAAE=";

describe("decode - sc_error", () => {
  it("decodes an error", () => {
    expect(decode(ERROR_CODE)).toEqual({
      type: "sc_error",
      value: "<error>",
      human: "<error>",
    });
  });
});

// ScVal u128: i32 discriminant 7 (SCV_U128) followed by a 16-byte big-endian u128.
const U128_ZERO = "AAAABwAAAAAAAAAAAAAAAAAAAAA=";
const U128_SMALL = "AAAABwAAAAAAAAAAAAAAAAAAACo=";

describe("decode - u128", () => {
  it("decodes u128 value 0 correctly", () => {
    expect(decode(U128_ZERO)).toEqual({
      type: "u128",
      value: "0",
      human: "0",
    });
  });

  it("decodes u128 value 42 correctly", () => {
    expect(decode(U128_SMALL)).toEqual({
      type: "u128",
      value: "42",
      human: "42",
    });
  });
});

// ScVal bitset: i32 discriminant 11 (SCV_BITSET) encoded like u128.
const BITSET_ZERO = "AAAACwAAAAAAAAAAAAAAAAAAAAA=";
const BITSET_ONE = "AAAACwAAAAAAAAAAAAAAAAAAAAE=";

describe("decode - bitset", () => {
  it("decodes bitset 0 correctly", () => {
    expect(decode(BITSET_ZERO)).toEqual({
      type: "bitset",
      value: "0",
      human: "0",
    });
  });

  it("decodes bitset 1 correctly", () => {
    expect(decode(BITSET_ONE)).toEqual({
      type: "bitset",
      value: "1",
      human: "1",
    });
  });
});

// ScVal string: i32 discriminant 12 (SCV_STRING) followed by a length-prefixed,
// 4-byte-padded string.
const STRING_EMPTY = "AAAADAAAAAA=";
const STRING_HELLO = "AAAADAAAAAVoZWxsbwAAAA==";

describe("decode - string", () => {
  it("decodes an empty string", () => {
    expect(decode(STRING_EMPTY)).toEqual({
      type: "string",
      value: "",
      human: "",
    });
  });

  it("decodes a string", () => {
    expect(decode(STRING_HELLO)).toEqual({
      type: "string",
      value: "hello",
      human: "hello",
    });
  });
});

// ScVal vec: i32 discriminant 13 (SCV_VEC) followed by length and elements.
const VEC_EMPTY = "AAAADQAAAAA=";
const VEC_123 = "AAAADQAAAAMAAAADAAAAAQAAAAMAAAACAAAAAwAAAAM=";

describe("decode - vec", () => {
  it("decodes an empty vec", () => {
    expect(decode(VEC_EMPTY)).toEqual({
      type: "vec",
      value: [],
      human: "[]",
    });
  });

  it("decodes a vec of u32 values", () => {
    expect(decode(VEC_123)).toEqual({
      type: "vec",
      value: [1, 2, 3],
      human: "[1,2,3]",
    });
  });
});

// ScVal map: i32 discriminant 14 (SCV_MAP) followed by length and key/value pairs.
const MAP_EMPTY = "AAAADgAAAAA=";
const MAP_AB = "AAAADgAAAAIAAAAKAAAAAWEAAAAAAAADAAAAAQAAAAoAAAABYgAAAAAAAAMAAAAC";

describe("decode - map", () => {
  it("decodes an empty map", () => {
    expect(decode(MAP_EMPTY)).toEqual({
      type: "map",
      value: {},
      human: "{}",
    });
  });

  it("decodes a symbol-to-u32 map", () => {
    expect(decode(MAP_AB)).toEqual({
      type: "map",
      value: { a: 1, b: 2 },
      human: '{"a":1,"b":2}',
    });
  });
});

// ScVal bytes: i32 discriminant 15 (SCV_BYTES) followed by length and raw bytes.
const BYTES_EMPTY = "AAAADwAAAAA=";
const BYTES_DEADBEEF = "AAAADwAAAATerb7v";

describe("decode - bytes", () => {
  it("decodes empty bytes", () => {
    expect(decode(BYTES_EMPTY)).toEqual({
      type: "bytes",
      value: "0x",
      human: "0x",
    });
  });

  it("decodes 0xdeadbeef bytes", () => {
    expect(decode(BYTES_DEADBEEF)).toEqual({
      type: "bytes",
      value: "0xdeadbeef",
      human: "0xdeadbeef",
    });
  });
});

// ScVal address: i32 discriminant 16 (SCV_ADDRESS) followed by 1 type byte + 32 key bytes.
const ADDRESS_ACCOUNT = "AAAAEACrq6urq6urq6urq6urq6urq6urq6urq6urq6urq6urqw==";
const ADDRESS_CONTRACT = "AAAAEAHNzc3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3NzQ==";

describe("decode - address", () => {
  it("decodes an account address", () => {
    const result = decode(ADDRESS_ACCOUNT);
    expect(result.type).toBe("address");
    expect(result.value).toContain("<account:");
    expect(result.human).toContain("<account:");
  });

  it("decodes a contract address", () => {
    const result = decode(ADDRESS_CONTRACT);
    expect(result.type).toBe("address");
    expect(result.value).toContain("<contract:");
    expect(result.human).toContain("<contract:");
  });
});

// ScVal contract_instance: i32 discriminant 17 (SCV_CONTRACT_INSTANCE).
const CONTRACT_INSTANCE = "AAAAEQ==";

describe("decode - contract_instance", () => {
  it("decodes a contract instance placeholder", () => {
    expect(decode(CONTRACT_INSTANCE)).toEqual({
      type: "contract_instance",
      value: "<contract_instance>",
      human: "<contract_instance>",
    });
  });
});

// ScVal ledger_key_instance: i32 discriminant 18 (SCV_LEDGER_KEY_CONTRACT_INSTANCE).
const LEDGER_KEY_INSTANCE = "AAAAEg==";

describe("decode - ledger_key_instance", () => {
  it("decodes a ledger key instance placeholder", () => {
    expect(decode(LEDGER_KEY_INSTANCE)).toEqual({
      type: "ledger_key_instance",
      value: "<ledger_key_instance>",
      human: "<ledger_key_instance>",
    });
  });
});

// ScVal ledger_key_nonce: i32 discriminant 19 (SCV_LEDGER_KEY_NONCE) followed by i64.
const NONCE_ZERO = "AAAAEwAAAAAAAAAA";
const NONCE_SMALL = "AAAAEwAAAAAAAAAq";

describe("decode - nonce", () => {
  it("decodes nonce 0 correctly", () => {
    expect(decode(NONCE_ZERO)).toEqual({
      type: "nonce",
      value: "0",
      human: "0",
    });
  });

  it("decodes nonce 42 correctly", () => {
    expect(decode(NONCE_SMALL)).toEqual({
      type: "nonce",
      value: "42",
      human: "42",
    });
  });
});

// ScVal time_point: i32 discriminant 20 (SCV_TIME_POINT) followed by u64.
const TIME_POINT = "AAAAFAAAAAAAAAAA";

describe("decode - time_point", () => {
  it("decodes time_point 0 correctly", () => {
    expect(decode(TIME_POINT)).toEqual({
      type: "time_point",
      value: "0",
      human: "0",
    });
  });
});

// ScVal duration: i32 discriminant 21 (SCV_DURATION) followed by u64.
const DURATION = "AAAAFQAAAAAAAAAA";

describe("decode - duration", () => {
  it("decodes duration 0 correctly", () => {
    expect(decode(DURATION)).toEqual({
      type: "duration",
      value: "0",
      human: "0",
    });
  });
});

describe("decodeScVal - bool", () => {
  it("decodes bool values to raw booleans", () => {
    expect(decodeScVal(BOOL_TRUE)).toBe(true);
    expect(decodeScVal(BOOL_FALSE)).toBe(false);
  });
});

// ScVal u64: i32 discriminant 5 (SCV_U64) followed by an 8-byte big-endian u64.
const U64_SMALL = "AAAABQAAAAAAAAAq"; // 42
const U64_MAX = "AAAABf//////////"; // 18446744073709551615 (u64 max)

describe("decode - u64", () => {
  it("decodes a small u64 value", () => {
    expect(decode(U64_SMALL)).toEqual({
      type: "u64",
      value: "42",
      human: "42",
    });
  });

  it("decodes a u64 value near max without precision loss", () => {
    expect(decode(U64_MAX)).toEqual({
      type: "u64",
      value: "18446744073709551615",
      human: "18446744073709551615",
    });
  });
});

// ScVal u32: i32 discriminant 3 (SCV_U32) followed by a 4-byte big-endian u32.
const U32_ZERO = "AAAAAwAAAAA=";
const U32_MAX = "AAAAA/////8="; // 4294967295 (u32 max)

describe("decode - u32", () => {
  it("decodes u32 value 0", () => {
    expect(decode(U32_ZERO)).toEqual({
      type: "u32",
      value: 0,
      human: "0",
    });
  });

  it("decodes u32 max value 4294967295", () => {
    expect(decode(U32_MAX)).toEqual({
      type: "u32",
      value: 4294967295,
      human: "4294967295",
    });
  });
});