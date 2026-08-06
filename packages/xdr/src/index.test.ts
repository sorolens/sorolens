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

// ScVal symbol: i32 discriminant 15 (SCV_SYMBOL) followed by a length-prefixed,
// 4-byte-padded string.
const SYMBOL_TRANSFER = "AAAADwAAAAhUcmFuc2Zlcg==";
const SYMBOL_MINT = "AAAADwAAAARtaW50";
const SYMBOL_EMPTY = "AAAADwAAAAA=";

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

describe("decodeScVal - bool", () => {
  it("decodes bool values to raw booleans", () => {
    expect(decodeScVal(BOOL_TRUE)).toBe(true);
    expect(decodeScVal(BOOL_FALSE)).toBe(false);
  });
});

// ScVal error: i32 discriminant 2 (SCV_ERROR) followed by the 8-byte ScError
// union (ScErrorType type + uint32 code).
const ERROR_SCVAL = "AAAAAgAAAAAAAAAA";

describe("decodeScVal - error", () => {
  it("decodes an error ScVal", () => {
    expect(decodeScVal(ERROR_SCVAL)).toBe("<error>");
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

describe("decodeScVal - u32", () => {
  it("decodes u32 values to raw numbers", () => {
    expect(decodeScVal(U32_ZERO)).toBe(0);
    expect(decodeScVal(U32_MAX)).toBe(4294967295);
  });
});

// ScVal 128/256-bit numerics: i32 discriminant (9=U128, 10=I128, 11=U256, 12=I256)
// followed by the big-endian value bytes.
const U128_ONE = "AAAACQAAAAAAAAAAAAAAAAAAAAE=";
const I128_NEG = "AAAACv////////////////////8=";
const U256_ONE = "AAAACwAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAB";
const I256_NEG = "AAAADP//////////////////////////////////////////";

// ScVal timepoint/duration: i32 discriminant (7/8) followed by an 8-byte u64.
const TIMEPOINT = "AAAABwAAAAAAAAAq";
const DURATION = "AAAACAAAAAAAAAAq";

// ScVal bytes/string: i32 discriminant (13/14) followed by a length-prefixed,
// 4-byte-padded payload.
const BYTES_DEAD = "AAAADQAAAALerQAA"; // 0xdead
const STRING_ABC = "AAAADgAAAANhYmMA"; // "abc"

// ScVal vec/map: i32 discriminant (16/17) followed by a count and entries.
const VEC_U32 = "AAAAEAAAAAIAAAADAAAABQAAAAMAAAAG"; // [5, 6]
const MAP_ONE = "AAAAEQAAAAEAAAAPAAAAAWsAAAAAAAADAAAABw=="; // { k: 7 }

// ScVal address: i32 discriminant 18 (SCV_ADDRESS) followed by the SCAddress
// union (4-byte ScAddressType discriminant + 32-byte payload).
const ADDR_ACCOUNT = "AAAAEgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==";
const ADDR_CONTRACT = "AAAAEgAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==";

const ZERO_HEX_32 = "0".repeat(64);

describe("decodeScVal - 128/256-bit numerics", () => {
  it("decodes u128", () => {
    expect(decodeScVal(U128_ONE)).toBe(1n);
  });

  it("decodes i128 (negative)", () => {
    expect(decodeScVal(I128_NEG)).toBe(-1n);
  });

  it("decodes u256", () => {
    expect(decodeScVal(U256_ONE)).toBe(1n);
  });

  it("decodes i256 (negative)", () => {
    expect(decodeScVal(I256_NEG)).toBe(-1n);
  });
});

describe("decodeScVal - timepoint and duration", () => {
  it("decodes a timepoint as u64", () => {
    expect(decodeScVal(TIMEPOINT)).toBe(42n);
  });

  it("decodes a duration as u64", () => {
    expect(decodeScVal(DURATION)).toBe(42n);
  });
});

describe("decodeScVal - bytes and string", () => {
  it("decodes bytes as hex", () => {
    expect(decodeScVal(BYTES_DEAD)).toBe("0xdead");
  });

  it("decodes a string", () => {
    expect(decodeScVal(STRING_ABC)).toBe("abc");
  });
});

describe("decodeScVal - collections", () => {
  it("decodes a vec", () => {
    expect(decodeScVal(VEC_U32)).toEqual([5, 6]);
  });

  it("decodes a map", () => {
    expect(decodeScVal(MAP_ONE)).toEqual({ k: 7 });
  });
});

describe("decodeScVal - address", () => {
  it("decodes an account address", () => {
    expect(decodeScVal(ADDR_ACCOUNT)).toBe("<account:" + ZERO_HEX_32 + ">");
  });

  it("decodes a contract address", () => {
    expect(decodeScVal(ADDR_CONTRACT)).toBe("<contract:" + ZERO_HEX_32 + ">");
  });
});