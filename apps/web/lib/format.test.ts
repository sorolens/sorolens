/**
 * Tests for apps/web/lib/format.ts
 *
 * We use vitest. The helper is pure, so no DOM is required beyond the shared
 * jsdom environment configured in vitest.config.ts.
 */

import { describe, expect, it } from "vitest";
import {
  CONTRACT_ID_HEAD_CHARS,
  CONTRACT_ID_TAIL_CHARS,
  truncateMiddle,
} from "./format";

// A real-shaped Soroban contract ID (56 chars, 'C' + 55 base32 chars).
const SOROBAN_CONTRACT_ID =
  "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGQFW2J";

describe("truncateMiddle", () => {
  it("returns short values unchanged", () => {
    expect(truncateMiddle("CABC")).toBe("CABC");
  });

  it("returns empty values unchanged", () => {
    expect(truncateMiddle("")).toBe("");
  });

  it("returns values that exactly fit unchanged", () => {
    // 8 head + 8 tail + 3 ellipsis characters === 19.
    const exact = "C".repeat(
      CONTRACT_ID_HEAD_CHARS + CONTRACT_ID_TAIL_CHARS + 3,
    );
    expect(exact).toHaveLength(19);
    expect(truncateMiddle(exact)).toBe(exact);
  });

  it("truncates long values in the middle", () => {
    const value = "C".repeat(10) + "D".repeat(10) + "E".repeat(10);
    expect(truncateMiddle(value)).toBe("CCCCCCCC...EEEEEEEE");
  });

  it("renders a 56-character Soroban contract ID in the shared shape", () => {
    expect(SOROBAN_CONTRACT_ID).toHaveLength(56);
    expect(truncateMiddle(SOROBAN_CONTRACT_ID)).toBe(
      "CDLZFC3S...HAGQFW2J",
    );
  });

  it("honours custom head and tail lengths", () => {
    const value = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGQFW2J";
    expect(truncateMiddle(value, 24, 8)).toBe("CDLZFC3SYJYDZT7K67VZ75HP...HAGQFW2J");
    expect(truncateMiddle(value, 4, 4)).toBe("CDLZ...FW2J");
  });
});
