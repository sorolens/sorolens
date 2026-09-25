import { describe, expect, it } from "vitest";
import {
  COMPARE_METRICS,
  bestFor,
  deltaTone,
  metricValue,
  worsePercent,
} from "./compare";
import type { CompareContractEntry } from "./types";

function entry(overrides: Partial<CompareContractEntry>): CompareContractEntry {
  return {
    id: "C1",
    network: "testnet",
    label: "",
    status: "active",
    tracked: true,
    has_data: true,
    event_count: 0,
    invocation_count: 0,
    avg_cpu: 0,
    avg_fee: 0,
    health_score: null,
    last_synced_ledger: 0,
    event_volume: [],
    ...overrides,
  };
}

const eventSpec = COMPARE_METRICS.find((m) => m.key === "event_count")!;
const cpuSpec = COMPARE_METRICS.find((m) => m.key === "avg_cpu")!;
const healthSpec = COMPARE_METRICS.find((m) => m.key === "health_score")!;

describe("deltaTone (higher is better)", () => {
  it("flags >20% worse as warning and >50% worse as danger", () => {
    expect(deltaTone(100, 100, true)).toBe("neutral");
    expect(deltaTone(85, 100, true)).toBe("neutral"); // 15% worse
    expect(deltaTone(79, 100, true)).toBe("warning"); // 21% worse
    expect(deltaTone(75, 100, true)).toBe("warning"); // 25% worse
    expect(deltaTone(49, 100, true)).toBe("danger"); // 51% worse
  });
});

describe("deltaTone (lower is better)", () => {
  it("treats a higher value as worse", () => {
    expect(deltaTone(100, 100, false)).toBe("neutral");
    expect(deltaTone(110, 100, false)).toBe("neutral"); // 10% worse
    expect(deltaTone(125, 100, false)).toBe("warning"); // 25% worse
    expect(deltaTone(160, 100, false)).toBe("danger"); // 60% worse
  });
});

describe("deltaTone edge cases", () => {
  it("is neutral when the best value is missing or non-positive", () => {
    expect(deltaTone(10, null, true)).toBe("neutral");
    expect(deltaTone(null, 100, true)).toBe("neutral");
    expect(deltaTone(5, 0, true)).toBe("neutral");
  });
});

describe("worsePercent", () => {
  it("returns the ratio only when the value is actually worse", () => {
    expect(worsePercent(50, 100, true)).toBeCloseTo(0.5);
    expect(worsePercent(120, 100, false)).toBeCloseTo(0.2);
    expect(worsePercent(100, 100, true)).toBeNull();
    expect(worsePercent(150, 100, true)).toBeNull(); // better, not worse
    expect(worsePercent(null, 100, true)).toBeNull();
  });
});

describe("bestFor", () => {
  const entries = [
    entry({ id: "a", event_count: 5, avg_cpu: 300 }),
    entry({ id: "b", event_count: 40, avg_cpu: 100 }),
    entry({ id: "c", has_data: false, event_count: 0, avg_cpu: 0 }),
  ];

  it("picks the max for higher-is-better metrics", () => {
    expect(bestFor(eventSpec, entries)).toBe(40);
  });

  it("picks the min for lower-is-better metrics", () => {
    expect(bestFor(cpuSpec, entries)).toBe(100);
  });

  it("ignores contracts with no data", () => {
    const onlyNoData = [entry({ id: "x", has_data: false, event_count: 999 })];
    expect(bestFor(eventSpec, onlyNoData)).toBeNull();
  });
});

describe("metricValue", () => {
  it("returns null for a contract with no data", () => {
    expect(metricValue(entry({ has_data: false, event_count: 7 }), eventSpec)).toBeNull();
  });

  it("returns null for an uncomputed health score", () => {
    expect(metricValue(entry({ health_score: null }), healthSpec)).toBeNull();
    expect(metricValue(entry({ health_score: 0 }), healthSpec)).toBe(0);
  });
});
