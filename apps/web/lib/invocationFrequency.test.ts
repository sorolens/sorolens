/**
 * Tests for apps/web/lib/invocationFrequency.ts (issue #185).
 *
 * vitest + jsdom. @/lib/api is mocked so paging is deterministic.
 */

import { describe, expect, it, vi, beforeEach } from "vitest";
import {
  aggregateInvocationFrequency,
  getInvocationFrequency,
} from "./invocationFrequency";
import type { Invocation } from "@/lib/types";

const mockGetContractInvocations = vi.fn();

vi.mock("@/lib/api", () => ({
  getContractInvocations: (...args: unknown[]) =>
    mockGetContractInvocations(...args),
}));

function invocation(ledgerClosedAt: string): Invocation {
  return {
    tx_hash: "a1b2c3",
    contract_id: "CABC1234",
    network: "testnet",
    ledger: 120_400,
    ledger_closed_at: ledgerClosedAt,
    status: "success",
    function_name: "transfer",
    args_decoded: null,
    result_decoded: null,
    resource_fee_charged: 100,
    cpu_insn: 1000,
    mem_byte: 500,
    ledger_read_byte: 0,
    ledger_write_byte: 0,
  };
}

describe("aggregateInvocationFrequency", () => {
  const NOW = new Date("2026-07-03T10:30:00Z");

  it("counts invocations per UTC hour", () => {
    const points = aggregateInvocationFrequency(
      [
        invocation("2026-07-03T09:05:00Z"),
        invocation("2026-07-03T09:45:00Z"),
        invocation("2026-07-03T10:00:01Z"),
      ],
      24,
      NOW
    );

    expect(points).toHaveLength(24);
    const byHour = new Map(points.map((p) => [p.hour, p.count]));
    expect(byHour.get("09:00")).toBe(2);
    expect(byHour.get("10:00")).toBe(1);
  });

  it("zero-fills hours with no invocations", () => {
    const points = aggregateInvocationFrequency(
      [invocation("2026-07-03T08:10:00Z")],
      24,
      NOW
    );

    expect(points).toHaveLength(24);
    expect(points.filter((p) => p.count === 0)).toHaveLength(23);
  });

  it("returns the oldest hour first and the current hour last", () => {
    const points = aggregateInvocationFrequency([], 24, NOW);

    // NOW is 10:30, so the current bucket starts at 10:00 and the window
    // starts 23 hours earlier at 11:00 the previous day.
    expect(points[0].hour).toBe("11:00");
    expect(points[23].hour).toBe("10:00");
    expect(points.map((p) => p.count).every((c) => c === 0)).toBe(true);
  });

  it("handles the singular hour label boundary (wraps midnight)", () => {
    const points = aggregateInvocationFrequency(
      [invocation("2026-07-04T00:20:00Z")],
      24,
      new Date("2026-07-04T01:15:00Z")
    );

    const byHour = new Map(points.map((p) => [p.hour, p.count]));
    expect(byHour.get("00:00")).toBe(1);
  });
});

describe("getInvocationFrequency", () => {
  beforeEach(() => {
    mockGetContractInvocations.mockReset();
  });

  it("aggregates invocations fetched with a 24-hour since bound", async () => {
    mockGetContractInvocations.mockResolvedValue({
      invocations: [invocation(new Date().toISOString())],
      next_cursor: "",
    });

    const points = await getInvocationFrequency("CCONTRACT", 24);

    expect(points).toHaveLength(24);
    const call = mockGetContractInvocations.mock.calls[0];
    expect(call[0]).toBe("CCONTRACT");
    const params = call[1] as { since: string };
    const ageMs = Date.now() - new Date(params.since).getTime();
    expect(ageMs).toBeGreaterThan(23 * 60 * 60 * 1000);
    expect(ageMs).toBeLessThan(25 * 60 * 60 * 1000);
  });

  it("pages through invocations and merges the results", async () => {
    const hourAgo = new Date(Date.now() - 60 * 60 * 1000).toISOString();
    const twoHoursAgo = new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString();
    mockGetContractInvocations
      .mockResolvedValueOnce({
        invocations: [invocation(twoHoursAgo)],
        next_cursor: "next",
      })
      .mockResolvedValueOnce({
        invocations: [invocation(hourAgo)],
        next_cursor: "",
      });

    const points = await getInvocationFrequency("CCONTRACT", 24);

    expect(mockGetContractInvocations).toHaveBeenCalledTimes(2);
    const total = points.reduce((sum, p) => sum + p.count, 0);
    expect(total).toBe(2);
    const secondCall = mockGetContractInvocations.mock.calls[1];
    expect((secondCall[1] as { cursor?: string }).cursor).toBe("next");
  });
});
