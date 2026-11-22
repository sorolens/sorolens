import { describe, expect, it } from "vitest";
import { contractOgCard, formatCount, shortContractId } from "@/lib/og";

const ID = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC";

describe("shortContractId", () => {
  it("truncates long contract IDs", () => {
    expect(shortContractId(ID)).toBe("CDLZFC…GCYSC");
  });

  it("keeps short IDs intact", () => {
    expect(shortContractId("CABC")).toBe("CABC");
  });
});

describe("formatCount", () => {
  it("formats small numbers verbatim", () => {
    expect(formatCount(42)).toBe("42");
  });

  it("abbreviates thousands and millions", () => {
    expect(formatCount(1500)).toBe("1.5K");
    expect(formatCount(2_400_000)).toBe("2.4M");
  });
});

describe("contractOgCard", () => {
  it("renders contract label, network and stats", () => {
    const card = contractOgCard({
      id: ID,
      label: "Aqua Token",
      network: "testnet",
      stats: {
        event_count: 1200,
        invocation_count: 340,
        storage_count: 12,
        last_synced_ledger: 100,
        window_event_count: 1200,
        window_invocation_count: 340,
        window_duration: "7d",
      },
    });

    expect(card.title).toBe("Aqua Token");
    expect(card.subtitle).toContain("testnet");
    expect(card.stats).toEqual([
      { label: "Events", value: "1.2K" },
      { label: "Invocations", value: "340" },
      { label: "Storage", value: "12" },
    ]);
    expect(card.alt).toContain("Aqua Token");
  });

  it("falls back gracefully without contract data", () => {
    const card = contractOgCard({ id: ID, label: null, network: null });

    expect(card.title).toBe("Contract CDLZFC…GCYSC");
    expect(card.subtitle).toBe("Soroban contract");
    expect(card.stats).toEqual([]);
    expect(card.alt).toContain("Contract CDLZFC…GCYSC");
  });
});
