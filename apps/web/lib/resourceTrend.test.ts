/**
 * Tests for apps/web/lib/resourceTrend.ts (issue #184, server-side per #600).
 *
 * vitest + jsdom. @/lib/api is mocked so the single request is deterministic.
 */

import { describe, expect, it, vi, beforeEach } from "vitest";
import { getResourceTrend } from "./resourceTrend";
import type { ResourceTrendPoint } from "@/lib/types";

const mockGetContractResourceTrend = vi.fn();

vi.mock("@/lib/api", () => ({
  getContractResourceTrend: (...args: unknown[]) =>
    mockGetContractResourceTrend(...args),
}));

describe("getResourceTrend", () => {
  beforeEach(() => {
    mockGetContractResourceTrend.mockReset();
  });

  it("fetches bucketed trend from the server in one request", async () => {
    const points: ResourceTrendPoint[] = [
      {
        date: "2026-07-03",
        avg_cpu_insn: 2000,
        avg_mem_byte: 600,
        avg_fee: 150,
        count: 2,
      },
    ];
    mockGetContractResourceTrend.mockResolvedValue(points);

    const data = await getResourceTrend("CCONTRACT", 30);

    expect(data).toEqual(points);
    expect(mockGetContractResourceTrend).toHaveBeenCalledTimes(1);
    expect(mockGetContractResourceTrend).toHaveBeenCalledWith("CCONTRACT", 30);
  });

  it("defaults to a 30-day window", async () => {
    mockGetContractResourceTrend.mockResolvedValue([]);

    await getResourceTrend("CCONTRACT");

    expect(mockGetContractResourceTrend).toHaveBeenCalledTimes(1);
    expect(mockGetContractResourceTrend).toHaveBeenCalledWith("CCONTRACT", 30);
  });

  it("returns the server's data unchanged (aggregation is server-side)", async () => {
    const points: ResourceTrendPoint[] = [
      {
        date: "2026-07-04",
        avg_cpu_insn: 500,
        avg_mem_byte: 100,
        avg_fee: 50,
        count: 1,
      },
    ];
    mockGetContractResourceTrend.mockResolvedValue(points);

    expect(await getResourceTrend("CCONTRACT", 7)).toEqual(points);
  });
});
