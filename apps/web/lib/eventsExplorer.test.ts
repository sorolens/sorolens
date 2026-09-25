import { describe, expect, it } from "vitest";
import {
  FIRST_PAGE,
  currentCursor,
  dateRangeError,
  dateRangeToBounds,
  formatEventData,
  pageNumber,
  popPage,
  previewEventData,
  pushPage,
} from "./eventsExplorer";

describe("cursor history", () => {
  it("walks forward and back through pages", () => {
    let h = FIRST_PAGE;
    expect(currentCursor(h)).toBe("");
    expect(pageNumber(h)).toBe(1);

    h = pushPage(h, "c2");
    h = pushPage(h, "c3");
    expect(currentCursor(h)).toBe("c3");
    expect(pageNumber(h)).toBe(3);

    h = popPage(h);
    expect(currentCursor(h)).toBe("c2");
    h = popPage(h);
    expect(currentCursor(h)).toBe("");
  });

  it("never pops past the first page or pushes an empty cursor", () => {
    expect(popPage(FIRST_PAGE)).toEqual([""]);
    expect(pushPage(FIRST_PAGE, "")).toEqual([""]);
  });
});

describe("dateRangeToBounds", () => {
  it("expands days to inclusive UTC bounds", () => {
    expect(dateRangeToBounds("2026-09-01", "2026-09-03")).toEqual({
      since: "2026-09-01T00:00:00Z",
      until: "2026-09-03T23:59:59Z",
    });
  });

  it("omits empty or malformed values", () => {
    expect(dateRangeToBounds("", "")).toEqual({});
    expect(dateRangeToBounds("09/01/2026", "2026-09-03")).toEqual({
      until: "2026-09-03T23:59:59Z",
    });
  });

  it("flags an inverted range", () => {
    expect(dateRangeError("2026-09-05", "2026-09-01")).toMatch(/start date/);
    expect(dateRangeError("2026-09-01", "2026-09-01")).toBeNull();
    expect(dateRangeError("", "2026-09-01")).toBeNull();
  });
});

describe("event data formatting", () => {
  it("pretty-prints nested values", () => {
    expect(formatEventData({ amount: 5, to: ["G1"] })).toBe(
      '{\n  "amount": 5,\n  "to": [\n    "G1"\n  ]\n}',
    );
    expect(formatEventData(null)).toBe("null");
    expect(formatEventData(BigInt(7))).toBe('"7"');
  });

  it("previews on one line and truncates", () => {
    expect(previewEventData({ a: 1 })).toBe('{ "a": 1 }');
    expect(previewEventData("x".repeat(100), 10)).toBe(`"${"x".repeat(8)}…`);
  });
});
