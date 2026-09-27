/**
 * Tests for apps/web/hooks/useContracts.ts
 *
 * `@/lib/api` is mocked so the hooks run against controlled responses. Each
 * render mounts a fresh SWR cache, so cached data never leaks between tests.
 */

import { renderHook, waitFor, act, cleanup } from "@testing-library/react";
import { SWRConfig } from "swr";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type {
  ContractDetail,
  ContractSummary,
  ContractsListResponse,
} from "@/lib/types";

const mockListContracts = vi.fn();
const mockGetContract = vi.fn();

vi.mock("@/lib/api", () => ({
  listContracts: (...args: unknown[]) => mockListContracts(...args),
  getContract: (...args: unknown[]) => mockGetContract(...args),
}));

function makeWrapper(dedupingInterval: number) {
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return (
      <SWRConfig
        value={{
          provider: () => new Map(),
          dedupingInterval,
          revalidateOnFocus: false,
          revalidateOnReconnect: false,
          shouldRetryOnError: false,
        }}
      >
        {children}
      </SWRConfig>
    );
  };
}

// Most tests disable de-duping so a revalidation is observable immediately.
const wrapper = makeWrapper(0);
// The dedupe test needs a window in which concurrent requests can collapse.
const dedupeWrapper = makeWrapper(2000);

const contractA: ContractSummary = {
  id: "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
  network: "testnet",
  label: "My Contract",
  status: "active",
  wasm_hash: null,
  added_at: "2024-01-01T00:00:00Z",
  last_activity_at: null,
  tags: [],
};

const contractB: ContractSummary = {
  ...contractA,
  id: "CBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
  network: "mainnet",
  label: "Other",
  status: "backfilling",
};

const listFixture: ContractsListResponse = {
  contracts: [contractA, contractB],
  cursor: null,
  has_more: false,
};

const detailFixture: ContractDetail = {
  id: contractA.id,
  network: contractA.network,
  label: contractA.label,
  status: contractA.status,
  wasm_hash: null,
  backfill_complete_at: null,
  sync: null,
  storage_entry_count: 0,
  expiring_entry_count: 0,
  tags: [],
  added_at: contractA.added_at,
};

describe("useContracts", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListContracts.mockResolvedValue(listFixture);
    mockGetContract.mockResolvedValue(detailFixture);
  });

  afterEach(() => {
    cleanup();
    vi.resetAllMocks();
  });

  it("fetches the list and exposes the typed payload", async () => {
    const { useContracts } = await import("@/hooks/useContracts");
    const { result } = renderHook(
      () => useContracts({ limit: 20, network: "testnet" }),
      { wrapper }
    );

    await waitFor(() => expect(result.current.data?.contracts).toHaveLength(2));
    expect(mockListContracts).toHaveBeenCalledWith({
      limit: 20,
      network: "testnet",
    });
  });

  it("de-duplicates concurrent requests for the same query", async () => {
    const { useContracts } = await import("@/hooks/useContracts");
    renderHook(
      () => ({
        a: useContracts({ limit: 20 }),
        b: useContracts({ limit: 20 }),
      }),
      { wrapper: dedupeWrapper }
    );

    await waitFor(() => expect(mockListContracts).toHaveBeenCalledTimes(1));
    // Give any duplicate request a chance to appear before asserting.
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(mockListContracts).toHaveBeenCalledTimes(1);
  });

  it("keys cache entries by query so different filters fetch separately", async () => {
    const { useContracts } = await import("@/hooks/useContracts");
    const { result } = renderHook(
      () => ({
        a: useContracts({ limit: 20, tag: "alpha" }),
        b: useContracts({ limit: 20, tag: "beta" }),
      }),
      { wrapper }
    );

    await waitFor(() => {
      expect(result.current.a.data?.contracts).toHaveLength(2);
      expect(result.current.b.data?.contracts).toHaveLength(2);
    });

    expect(mockListContracts).toHaveBeenCalledWith({
      limit: 20,
      tag: "alpha",
    });
    expect(mockListContracts).toHaveBeenCalledWith({ limit: 20, tag: "beta" });
  });

  it("serves stale data while revalidating (stale-while-revalidate)", async () => {
    const { useContracts } = await import("@/hooks/useContracts");
    const { result } = renderHook(() => useContracts({ limit: 20 }), {
      wrapper,
    });
    await waitFor(() =>
      expect(result.current.data?.contracts[0].label).toBe("My Contract")
    );

    // Hold the refresh open so the stale payload is observable.
    let resolveRefresh!: (value: ContractsListResponse) => void;
    mockListContracts.mockImplementationOnce(
      () => new Promise<ContractsListResponse>((res) => (resolveRefresh = res))
    );

    let pending!: Promise<unknown>;
    await act(async () => {
      pending = result.current.mutate();
      // Let the revalidation start without awaiting the held request.
      await Promise.resolve();
    });

    // The background request is in flight while the cached page is shown.
    expect(mockListContracts).toHaveBeenCalledTimes(2);
    expect(result.current.data?.contracts[0].label).toBe("My Contract");

    await act(async () => {
      resolveRefresh({ ...listFixture, contracts: [contractB] });
      await pending;
    });

    await waitFor(() =>
      expect(result.current.data?.contracts[0].label).toBe("Other")
    );
  });
});

describe("useContract", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetContract.mockResolvedValue(detailFixture);
  });

  afterEach(() => {
    cleanup();
    vi.resetAllMocks();
  });

  it("fetches a single contract by id", async () => {
    const { useContract } = await import("@/hooks/useContracts");
    const { result } = renderHook(() => useContract(contractA.id), { wrapper });

    await waitFor(() => expect(result.current.data?.id).toBe(contractA.id));
    expect(mockGetContract).toHaveBeenCalledWith(contractA.id);
  });

  it("does not fetch when the id is falsy", async () => {
    const { useContract } = await import("@/hooks/useContracts");
    const { result } = renderHook(() => useContract(null), { wrapper });

    expect(result.current.data).toBeUndefined();
    expect(mockGetContract).not.toHaveBeenCalled();
  });
});
