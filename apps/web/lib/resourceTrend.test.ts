/**
 * Tests for apps/web/lib/resourceTrend.ts (issues #184, #600).
 *
 * The chart must load from the server aggregation endpoint in one request,
 * not by paging through raw invocations.
 */

import { describe, expect, it, vi, beforeEach } from "vitest";
import { getResourceTrend } from "./resourceTrend";

const mockGetContractResourceTrend = vi.fn();

vi.mock("@/lib/api", () => ({
  getContractResourceTrend: (...args: unknown[]) =>
    mockGetContractResourceTrend(...args),
}));

describe("getResourceTrend", () => {
  beforeEach(() => {
    mockGetContractResourceTrend.mockReset();
  });

  it("returns the server buckets from one request", async () => {
    mockGetContractResourceTrend.mockResolvedValue({
      days: 30,
      points: [
        {
          date: "2026-07-03",
          avg_cpu_insn: 2000,
          avg_mem_byte: 600,
          avg_fee: 150,
          count: 2,
        },
      ],
    });

    const points = await getResourceTrend("CCONTRACT", 30);

    expect(mockGetContractResourceTrend).toHaveBeenCalledTimes(1);
    expect(mockGetContractResourceTrend).toHaveBeenCalledWith("CCONTRACT", 30);
    expect(points).toEqual([
      {
        date: "2026-07-03",
        avg_cpu_insn: 2000,
        avg_mem_byte: 600,
        avg_fee: 150,
        count: 2,
      },
    ]);
  });

  it("defaults to a 30-day window", async () => {
    mockGetContractResourceTrend.mockResolvedValue({ days: 30, points: [] });

    await getResourceTrend("CCONTRACT");

    expect(mockGetContractResourceTrend).toHaveBeenCalledWith("CCONTRACT", 30);
  });
});
