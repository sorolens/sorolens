/**
 * Tests for apps/web/lib/resourceTrend.ts (issue #184).
 *
 * vitest + jsdom. @/lib/api is mocked so paging is deterministic.
 */

import { describe, expect, it, vi, beforeEach } from "vitest";
import { aggregateResourceTrend, getResourceTrend } from "./resourceTrend";
import type { Invocation } from "@/lib/types";

const mockGetContractInvocations = vi.fn();

vi.mock("@/lib/api", () => ({
  getContractInvocations: (...args: unknown[]) =>
    mockGetContractInvocations(...args),
}));

function invocation(overrides: Partial<Invocation>): Invocation {
  return {
    tx_hash: "a1b2c3",
    ledger: 120_400,
    ledger_closed_at: "2026-07-03T08:00:00Z",
    status: "success",
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

describe("aggregateResourceTrend", () => {
  it("averages CPU, memory and fee per UTC day", () => {
    const points = aggregateResourceTrend([
      invocation({
        ledger_closed_at: "2026-07-03T08:00:00Z",
        cpu_insn: 1000,
        mem_byte: 500,
        resource_fee_charged: 100,
      }),
      invocation({
        ledger_closed_at: "2026-07-03T23:59:59Z",
        cpu_insn: 3000,
        mem_byte: 700,
        resource_fee_charged: 200,
      }),
      invocation({
        ledger_closed_at: "2026-07-04T00:00:01Z",
        cpu_insn: 500,
        mem_byte: 100,
        resource_fee_charged: 50,
      }),
    ]);

    expect(points).toEqual([
      {
        date: "2026-07-03",
        avg_cpu_insn: 2000,
        avg_mem_byte: 600,
        avg_fee: 150,
        count: 2,
      },
      {
        date: "2026-07-04",
        avg_cpu_insn: 500,
        avg_mem_byte: 100,
        avg_fee: 50,
        count: 1,
      },
    ]);
  });

  it("sorts days ascending regardless of input order", () => {
    const points = aggregateResourceTrend([
      invocation({ ledger_closed_at: "2026-07-05T10:00:00Z" }),
      invocation({ ledger_closed_at: "2026-07-01T10:00:00Z" }),
    ]);

    expect(points.map((p) => p.date)).toEqual(["2026-07-01", "2026-07-05"]);
  });

  it("returns an empty array for no invocations", () => {
    expect(aggregateResourceTrend([])).toEqual([]);
  });
});

describe("getResourceTrend", () => {
  beforeEach(() => {
    mockGetContractInvocations.mockReset();
  });

  it("aggregates invocations fetched with a 30-day since bound", async () => {
    mockGetContractInvocations.mockResolvedValue({
      invocations: [invocation({ ledger_closed_at: "2026-07-03T08:00:00Z" })],
      cursor: null,
      has_more: false,
    });

    const points = await getResourceTrend("CCONTRACT", 30);

    expect(points).toHaveLength(1);
    const call = mockGetContractInvocations.mock.calls[0];
    expect(call[0]).toBe("CCONTRACT");
    const params = call[1] as { since: string };
    const ageMs = Date.now() - new Date(params.since).getTime();
    expect(ageMs).toBeGreaterThan(29 * 24 * 60 * 60 * 1000);
    expect(ageMs).toBeLessThan(31 * 24 * 60 * 60 * 1000);
  });

  it("pages through invocations and merges the results", async () => {
    mockGetContractInvocations
      .mockResolvedValueOnce({
        invocations: [invocation({ ledger_closed_at: "2026-07-01T08:00:00Z" })],
        cursor: "next",
        has_more: true,
      })
      .mockResolvedValueOnce({
        invocations: [invocation({ ledger_closed_at: "2026-07-02T08:00:00Z" })],
        cursor: null,
        has_more: false,
      });

    const points = await getResourceTrend("CCONTRACT", 30);

    expect(mockGetContractInvocations).toHaveBeenCalledTimes(2);
    expect(points.map((p) => p.date)).toEqual(["2026-07-01", "2026-07-02"]);
    const secondCall = mockGetContractInvocations.mock.calls[1];
    expect((secondCall[1] as { cursor?: string }).cursor).toBe("next");
  });
});
