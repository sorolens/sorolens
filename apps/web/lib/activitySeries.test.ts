/**
 * Tests for apps/web/lib/activitySeries.ts (issue #586).
 *
 * vitest + jsdom. @/lib/api is mocked so paging is deterministic.
 */

import { describe, expect, it, vi, beforeEach } from "vitest";
import {
  activityWindow,
  aggregateActivity,
  getActivitySeries,
} from "./activitySeries";
import type { Invocation, ContractEvent } from "@/lib/types";

const mockGetContractEvents = vi.fn();
const mockGetContractInvocations = vi.fn();

vi.mock("@/lib/api", () => ({
  getContractEvents: (...args: unknown[]) => mockGetContractEvents(...args),
  getContractInvocations: (...args: unknown[]) =>
    mockGetContractInvocations(...args),
}));

function event(overrides: Partial<ContractEvent>): ContractEvent {
  return {
    id: "evt",
    contract_id: "CCONTRACT",
    network: "testnet",
    ledger: 120_400,
    ledger_closed_at: "2026-07-10T09:00:00Z",
    tx_hash: "a1b2c3",
    type: "transfer",
    topic_decoded: null,
    topic_xdr: [],
    value_decoded: null,
    value_xdr: "",
    in_successful_call: true,
    ...overrides,
  };
}

function invocation(overrides: Partial<Invocation>): Invocation {
  return {
    tx_hash: "a1b2c3",
    contract_id: "CCONTRACT",
    network: "testnet",
    ledger: 120_400,
    ledger_closed_at: "2026-07-10T09:00:00Z",
    status: "SUCCESS",
    function_name: "transfer",
    args_decoded: null,
    result_decoded: null,
    resource_fee_charged: 100,
    cpu_insn: 1000,
    mem_byte: 500,
    ledger_read_byte: 0,
    ledger_write_byte: 0,
    ...overrides,
  };
}

const NOW = new Date("2026-07-10T12:34:00Z");

describe("activityWindow", () => {
  it("buckets the last 24 hourly buckets for 24h", () => {
    const spec = activityWindow("24h", NOW);
    expect(spec.bucket).toBe("hour");
    expect(spec.buckets).toBe(24);
    expect(spec.since?.toISOString()).toBe("2026-07-09T13:00:00.000Z");
  });

  it("buckets the last 7 and 30 days for 7d and 30d", () => {
    expect(activityWindow("7d", NOW)).toMatchObject({
      bucket: "day",
      buckets: 7,
    });
    expect(activityWindow("30d", NOW)).toMatchObject({
      bucket: "day",
      buckets: 30,
    });
    expect(activityWindow("7d", NOW).since?.toISOString()).toBe(
      "2026-07-04T00:00:00.000Z"
    );
  });

  it("leaves 'all' unbounded and without zero-fill", () => {
    expect(activityWindow("all", NOW)).toEqual({
      since: null,
      bucket: "day",
      buckets: null,
    });
  });
});

describe("aggregateActivity", () => {
  it("counts events per UTC day and zero-fills the window", () => {
    const points = aggregateActivity(
      [
        event({ ledger: 10, ledger_closed_at: "2026-07-04T00:00:00Z" }),
        event({ ledger: 20, ledger_closed_at: "2026-07-10T09:00:00Z" }),
        event({ ledger: 25, ledger_closed_at: "2026-07-10T20:00:00Z" }),
        // Outside the 7-day window.
        event({ ledger_closed_at: "2026-07-03T23:59:59Z" }),
      ],
      "7d",
      NOW
    );

    expect(points.map((p) => p.date)).toEqual([
      "2026-07-04",
      "2026-07-05",
      "2026-07-06",
      "2026-07-07",
      "2026-07-08",
      "2026-07-09",
      "2026-07-10",
    ]);
    expect(points[0]).toMatchObject({ count: 1, ledger: 10 });
    expect(points[6]).toMatchObject({ count: 2, ledger: 25 });
    expect(points.slice(1, 6).every((p) => p.count === 0)).toBe(true);
  });

  it("counts invocations per UTC hour for 24h", () => {
    const points = aggregateActivity(
      [
        invocation({ ledger: 1, ledger_closed_at: "2026-07-09T13:10:00Z" }),
        invocation({ ledger: 2, ledger_closed_at: "2026-07-10T12:05:00Z" }),
        invocation({ ledger: 3, ledger_closed_at: "2026-07-10T12:45:00Z" }),
      ],
      "24h",
      NOW
    );

    expect(points).toHaveLength(24);
    expect(points[0]).toMatchObject({
      date: "2026-07-09 13:00",
      count: 1,
    });
    expect(points[23]).toMatchObject({
      date: "2026-07-10 12:00",
      count: 2,
      ledger: 3,
    });
  });

  it("returns only populated days for 'all', oldest first", () => {
    const points = aggregateActivity(
      [
        event({ ledger: 30, ledger_closed_at: "2026-07-10T09:00:00Z" }),
        event({ ledger: 11, ledger_closed_at: "2026-07-01T10:00:00Z" }),
        event({ ledger: 12, ledger_closed_at: "2026-07-01T11:00:00Z" }),
      ],
      "all",
      NOW
    );

    expect(points).toEqual([
      { date: "2026-07-01", ledger: 12, count: 2 },
      { date: "2026-07-10", ledger: 30, count: 1 },
    ]);
  });

  it("ignores unparseable timestamps", () => {
    expect(
      aggregateActivity([event({ ledger_closed_at: "not-a-date" })], "7d", NOW)
    ).toHaveLength(7);
  });

  it("returns an empty series for an empty 'all' window", () => {
    expect(aggregateActivity([], "all", NOW)).toEqual([]);
  });
});

describe("getActivitySeries", () => {
  beforeEach(() => {
    mockGetContractEvents.mockReset();
    mockGetContractInvocations.mockReset();
  });

  it("pages while rows are inside the window", async () => {
    mockGetContractEvents
      .mockResolvedValueOnce({
        events: [event({ ledger_closed_at: "2026-07-10T09:00:00Z" })],
        next_cursor: "next-events",
      })
      .mockResolvedValueOnce({
        events: [event({ ledger_closed_at: "2026-07-08T09:00:00Z" })],
        next_cursor: "",
      });
    mockGetContractInvocations.mockResolvedValueOnce({
      invocations: [invocation({ ledger_closed_at: "2026-07-10T10:00:00Z" })],
      next_cursor: "",
    });

    const series = await getActivitySeries("CCONTRACT", "7d", NOW);

    expect(mockGetContractEvents).toHaveBeenCalledTimes(2);
    expect(
      (mockGetContractEvents.mock.calls[1][1] as { cursor?: string }).cursor
    ).toBe("next-events");
    expect(mockGetContractInvocations).toHaveBeenCalledTimes(1);
    expect(series.events[6].count).toBe(1);
    expect(series.events[4].count).toBe(1);
    expect(series.invocations[6].count).toBe(1);
  });

  it("stops as soon as a page ends before the window start", async () => {
    mockGetContractEvents.mockResolvedValueOnce({
      events: [event({ ledger_closed_at: "2026-06-01T09:00:00Z" })],
      next_cursor: "older",
    });
    mockGetContractInvocations.mockResolvedValueOnce({
      invocations: [invocation({ ledger_closed_at: "2026-06-01T09:00:00Z" })],
      next_cursor: "older",
    });

    const series = await getActivitySeries("CCONTRACT", "7d", NOW);

    expect(mockGetContractEvents).toHaveBeenCalledTimes(1);
    expect(mockGetContractInvocations).toHaveBeenCalledTimes(1);
    expect(series.events.every((p) => p.count === 0)).toBe(true);
    expect(series.invocations.every((p) => p.count === 0)).toBe(true);
  });
});
