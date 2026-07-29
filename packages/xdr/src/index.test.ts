import { describe, expect, it } from "vitest";
import { decodeScVal } from "./index";

describe("decodeScVal", () => {
  it("decodes ScVal bool(true)", () => {
    expect(decodeScVal("AAAAAAAAAAE=")).toBe(true);
  });

  it("decodes ScVal bool(false)", () => {
    expect(decodeScVal("AAAAAAAAAAA=")).toBe(false);
  });

  it("decodes a bool followed by another value without desyncing the offset", () => {
    // Vec [ bool(true), u32(42) ]
    expect(decodeScVal("AAAADQAAAAIAAAAAAAAAAQAAAAMAAAAq")).toEqual([
      true,
      42,
    ]);
  });
});
