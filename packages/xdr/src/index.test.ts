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

describe("decodeScVal - bool", () => {
  it("decodes bool values to raw booleans", () => {
    expect(decodeScVal(BOOL_TRUE)).toBe(true);
    expect(decodeScVal(BOOL_FALSE)).toBe(false);
  });
});
