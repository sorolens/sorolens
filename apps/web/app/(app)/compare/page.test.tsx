/**
 * Tests for app/(app)/compare/page.tsx URL parsing (issue #607).
 *
 * The server component's whole job is turning ?ids=…&window=… into validated
 * props for the client view, so the validators are tested directly.
 */

import { describe, expect, it } from "vitest";
import { parseIds, parseWindow } from "@/lib/compareParams";

// 56-char Soroban ids: "C" + 55 base32 characters.
const id = (letter: string) => "C" + letter.repeat(55);
const ID_A = id("A");
const ID_B = id("B");
const ID_C = id("C");
const ID_D = id("D");
const ID_E = id("E");

describe("parseIds", () => {
  it("returns an empty result for a missing or blank parameter", () => {
    expect(parseIds(undefined)).toEqual({
      ids: [],
      dropped: 0,
      truncated: 0,
    });
    expect(parseIds("  ,, ")).toEqual({ ids: [], dropped: 0, truncated: 0 });
  });

  it("keeps valid ids in order, trimmed and upper-cased", () => {
    const r = parseIds(` ${ID_A.toLowerCase()} , ${ID_B} `);
    expect(r.ids).toEqual([ID_A, ID_B]);
    expect(r.dropped).toBe(0);
    expect(r.truncated).toBe(0);
  });

  it("de-duplicates ids that differ only in case", () => {
    const r = parseIds(`${ID_A},${ID_A.toLowerCase()}`);
    expect(r.ids).toEqual([ID_A]);
  });

  it("drops malformed ids and counts them", () => {
    const r = parseIds(`${ID_A},not-a-contract-id,${ID_B},Cshort`);
    expect(r.ids).toEqual([ID_A, ID_B]);
    expect(r.dropped).toBe(2);
  });

  it("caps the selection at 4 and counts the extras", () => {
    const r = parseIds([ID_A, ID_B, ID_C, ID_D, ID_E].join(","));
    expect(r.ids).toEqual([ID_A, ID_B, ID_C, ID_D]);
    expect(r.truncated).toBe(1);
    expect(r.dropped).toBe(0);
  });
});

describe("parseWindow", () => {
  it("accepts every window the app defines, including all", () => {
    expect(parseWindow("24h")).toBe("24h");
    expect(parseWindow("7d")).toBe("7d");
    expect(parseWindow("30d")).toBe("30d");
    expect(parseWindow("all")).toBe("all");
  });

  it("falls back to 7d for unknown or missing values", () => {
    expect(parseWindow("1h")).toBe("7d");
    expect(parseWindow(undefined)).toBe("7d");
  });
});
