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