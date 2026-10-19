/**
 * Tests for lib/compareAll.ts (issue #607, "all"-window comparison entries).
 *
 * @/lib/api is mocked (keeping the real ApiError class for instanceof
 * checks); the activity-series paging is driven through the mocked events and
 * invocations endpoints so getActivitySeries runs for real against stub data.
 */

import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import type {
  ContractDetail,
  ContractEvent,
  ContractStats,
  HealthScoreResponse,
  Invocation,
} from "@/lib/types";

const mockGetContract = vi.fn();
const mockGetContractStats = vi.fn();
const mockGetContractHealthScore = vi.fn();
const mockGetContractEvents = vi.fn();
const mockGetContractInvocations = vi.fn();

vi.mock("@/lib/api", async (importActual) => {
  const actual = await importActual<typeof import("@/lib/api")>();
  return {
    ...actual,
    getContract: (...args: unknown[]) => mockGetContract(...args),
    getContractStats: (...args: unknown[]) => mockGetContractStats(...args),
    getContractHealthScore: (...args: unknown[]) =>
      mockGetContractHealthScore(...args),
    getContractEvents: (...args: unknown[]) => mockGetContractEvents(...args),
    getContractInvocations: (...args: unknown[]) =>
      mockGetContractInvocations(...args),
  };
});

import { getCompareAllEntries } from "./compareAll";

const ID_A = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA";
const ID_B = "CBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB";

function detail(id: string): ContractDetail {
  return {
    id,
    network: "testnet",
    label: "Alpha",
    status: "active",
    wasm_hash: null,
    created_at_ledger: 1,
    backfill_complete_at: null,
    tags: [],
    added_at: "2026-01-01T00:00:00Z",
  };
}

function totals(partial: Partial<ContractStats> = {}): ContractStats {
  return {
    event_count: 1234,
    invocation_count: 567,
    storage_count: 10,
    last_synced_ledger: 99,
    window_event_count: 10,
    window_invocation_count: 5,
    window_duration: "24h",
    ...partial,
  };
}

function dayEvent(ledger: number, iso: string): ContractEvent {
  return { ledger, ledger_closed_at: iso } as unknown as ContractEvent;
}

beforeEach(() => {
  vi.clearAllMocks();
  mockGetContract.mockResolvedValue(detail(ID_A));
  mockGetContractStats.mockResolvedValue(totals());
  mockGetContractHealthScore.mockResolvedValue({
    score: 88,
  } as unknown as HealthScoreResponse);
  mockGetContractEvents.mockResolvedValue({
    events: [
      dayEvent(100, "2026-01-05T10:00:00Z"),
      dayEvent(101, "2026-01-05T11:00:00Z"),
    ],
    next_cursor: null,
  });
  mockGetContractInvocations.mockResolvedValue({
    invocations: [] as Invocation[],
    next_cursor: null,
  });
});

describe("getCompareAllEntries", () => {
  it("builds an entry from lifetime totals and the full-range series", async () => {
    const [entry] = await getCompareAllEntries([ID_A]);

    expect(mockGetContractStats).toHaveBeenCalledWith(ID_A, "24h");
    expect(entry).toMatchObject({
      id: ID_A,
      network: "testnet",
      label: "Alpha",
      status: "active",
      tracked: true,
      has_data: true,
      // Totals, not window counts: the window fields are deliberately ignored.
      event_count: 1234,
      invocation_count: 567,
      last_synced_ledger: 99,
      health_score: 88,
    });
    // No endpoint exposes lifetime CPU/fee averages; they stay null so the
    // column renders "—" rather than a misleading zero.
    expect(entry.avg_cpu).toBeNull();
    expect(entry.avg_fee).toBeNull();
    // The sparkline comes from the day-bucketed "all" series.
    expect(entry.event_volume).toEqual([{ timestamp: "2026-01-05", count: 2 }]);
  });

  it("returns a zeroed untracked entry (mirroring the API) without more calls", async () => {
    mockGetContract.mockRejectedValue(new ApiError(404, "not found"));

    const [entry] = await getCompareAllEntries([ID_A]);

    expect(entry.tracked).toBe(false);
    expect(entry.has_data).toBe(false);
    expect(entry.status).toBe("unknown");
    expect(entry.error).toBeUndefined();
    expect(mockGetContractStats).not.toHaveBeenCalled();
    expect(mockGetContractHealthScore).not.toHaveBeenCalled();
  });

  it("flags a stats outage with a per-entry error instead of throwing", async () => {
    mockGetContractStats.mockRejectedValue(new ApiError(500, "boom"));

    const [entry] = await getCompareAllEntries([ID_A]);

    expect(entry.error).toBe("failed to load stats");
    expect(entry.tracked).toBe(true);
  });

  it("treats a missing health score as null but other failures as errors", async () => {
    mockGetContractHealthScore.mockRejectedValue(new ApiError(404, "no score"));
    const [missing] = await getCompareAllEntries([ID_A]);
    expect(missing.health_score).toBeNull();
    expect(missing.error).toBeUndefined();

    mockGetContractHealthScore.mockRejectedValue(new ApiError(500, "boom"));
    const [failed] = await getCompareAllEntries([ID_A]);
    expect(failed.error).toBe("failed to load health score");
  });

  it("keeps request order across multiple contracts", async () => {
    mockGetContract.mockImplementation(async (id: string) =>
      id === ID_B ? detail(ID_B) : Promise.reject(new ApiError(404, "gone"))
    );

    const entries = await getCompareAllEntries([ID_B, ID_A]);

    expect(entries.map((e) => e.id)).toEqual([ID_B, ID_A]);
    expect(entries[0].tracked).toBe(true);
    expect(entries[1].tracked).toBe(false);
  });
});
