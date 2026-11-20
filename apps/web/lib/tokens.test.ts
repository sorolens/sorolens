import { afterEach, describe, expect, it, vi } from "vitest";
import {
  filterHolders,
  filterTransfers,
  formatTokenAmount,
  hasTokenActivity,
  holderDistribution,
  holdersToCsv,
  parseTokenAmount,
  shareOfSupply,
  supplyTotal,
  transfersToCsv,
  type TokenTransfer,
} from "./tokens";

const transfer = (overrides: Partial<TokenTransfer> = {}): TokenTransfer => ({
  id: "1",
  kind: "transfer",
  from: "GAAA",
  to: "GBBB",
  amount: "25000000",
  tx_hash: "abc",
  ledger: 10,
  ledger_closed_at: "2026-07-03T08:00:00Z",
  ...overrides,
});

afterEach(() => vi.unstubAllGlobals());

describe("formatTokenAmount", () => {
  it("applies decimals and groups the whole part", () => {
    expect(formatTokenAmount("12345678900000", 7)).toBe("1,234,567.89");
  });

  it("keeps amounts smaller than one unit", () => {
    expect(formatTokenAmount("5", 7)).toBe("0.0000005");
  });

  it("handles zero decimals and negatives", () => {
    expect(formatTokenAmount("1000", 0)).toBe("1,000");
    expect(formatTokenAmount("-15000000", 7)).toBe("-1.5");
  });

  it("does not lose precision on i128-sized values", () => {
    expect(
      formatTokenAmount("170141183460469231731687303715884105727", 7)
    ).toBe("17,014,118,346,046,923,173,168,730,371,588.4105727");
  });

  it("returns malformed input unchanged", () => {
    expect(formatTokenAmount("n/a", 7)).toBe("n/a");
  });
});

describe("parseTokenAmount", () => {
  it("converts to raw units", () => {
    expect(parseTokenAmount("1.5", 7)).toBe(BigInt(15000000));
    expect(parseTokenAmount("1,000", 2)).toBe(BigInt(100000));
    expect(parseTokenAmount(".5", 1)).toBe(BigInt(5));
  });

  it("rejects invalid input and excess precision", () => {
    expect(parseTokenAmount("abc", 7)).toBeNull();
    expect(parseTokenAmount("", 7)).toBeNull();
    expect(parseTokenAmount("0.123", 2)).toBeNull();
  });
});

describe("shares of supply", () => {
  it("computes a percentage and guards an empty supply", () => {
    expect(shareOfSupply("250", BigInt(1000))).toBe(25);
    expect(shareOfSupply("250", BigInt(0))).toBe(0);
  });

  it("prefers the reported supply over the sum of loaded holders", () => {
    const holders = [{ address: "GA", balance: "10" }];
    expect(supplyTotal(holders, "100")).toBe(BigInt(100));
    expect(supplyTotal(holders)).toBe(BigInt(10));
  });

  it("rolls holders past the cutoff into Others", () => {
    const holders = [
      { address: "GAAAAAAAAA", balance: "100" },
      { address: "GBBBBBBBBB", balance: "500" },
      { address: "GCCCCCCCCC", balance: "400" },
    ];
    expect(holderDistribution(holders, BigInt(1000), 2)).toEqual([
      { label: "GBBB…BBBB", share: 50 },
      { label: "GCCC…CCCC", share: 40 },
      { label: "Others", share: 10 },
    ]);
  });
});

describe("filters", () => {
  it("matches transfers on either side, ignoring case", () => {
    const rows = [
      transfer({ id: "1" }),
      transfer({ id: "2", from: null, to: "GCCC", kind: "mint" }),
    ];
    const match = filterTransfers(rows, { address: "gbbb", minAmount: null });
    expect(match.map((t) => t.id)).toEqual(["1"]);
  });

  it("applies a minimum amount", () => {
    const rows = [
      transfer({ id: "1", amount: "10" }),
      transfer({ id: "2", amount: "20" }),
    ];
    const match = filterTransfers(rows, {
      address: "",
      minAmount: BigInt(20),
    });
    expect(match.map((t) => t.id)).toEqual(["2"]);
  });

  it("filters holders by address and balance", () => {
    const holders = [
      { address: "GAAA", balance: "10" },
      { address: "GBBB", balance: "20" },
    ];
    expect(
      filterHolders(holders, { address: "GA", minAmount: null })
    ).toHaveLength(1);
    expect(
      filterHolders(holders, { address: "", minAmount: BigInt(15) })
    ).toEqual([{ address: "GBBB", balance: "20" }]);
  });
});

describe("csv export", () => {
  it("writes formatted transfers without thousands separators", () => {
    const csv = transfersToCsv([transfer({ amount: "12345678900000" })], 7);
    expect(csv.split("\n")[1]).toBe(
      "transfer,GAAA,GBBB,1234567.89,abc,10,2026-07-03T08:00:00Z"
    );
  });

  it("neutralises spreadsheet formulas", () => {
    const csv = transfersToCsv([transfer({ from: "=HYPERLINK(1,2)" })], 7);
    expect(csv).toContain(`"'=HYPERLINK(1,2)"`);
  });

  it("writes holders with their share of supply", () => {
    const csv = holdersToCsv(
      [{ address: "GAAA", balance: "2500000" }],
      7,
      BigInt(10000000)
    );
    expect(csv.split("\n")).toEqual([
      "address,balance,share_of_supply_pct",
      "GAAA,0.25,25",
    ]);
  });
});

describe("hasTokenActivity", () => {
  it("is false while the endpoint is unavailable", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response("{}", { status: 404 }))
    );
    await expect(hasTokenActivity("C1")).resolves.toBe(false);
  });

  it("is false on a network failure", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("offline")));
    await expect(hasTokenActivity("C1")).resolves.toBe(false);
  });

  it("is false for a contract without transfers", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          new Response(JSON.stringify({ transfers: [], next_cursor: "" }))
        )
    );
    await expect(hasTokenActivity("C1")).resolves.toBe(false);
  });

  it("is true once a transfer exists", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          new Response(
            JSON.stringify({ transfers: [transfer()], next_cursor: "" })
          )
        )
    );
    await expect(hasTokenActivity("C1")).resolves.toBe(true);
  });
});
