import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

const { searchParams, searchContracts, resolveLabel, getInvocationTrace } =
  vi.hoisted(() => ({
    searchParams: new URLSearchParams(),
    searchContracts: vi.fn(),
    resolveLabel: vi.fn(),
    getInvocationTrace: vi.fn(),
  }));

vi.mock("next/navigation", () => ({
  useSearchParams: () => searchParams,
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
}));

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
  }: {
    children: React.ReactNode;
    href: string;
  }) => <a href={href}>{children}</a>,
}));

vi.mock("@/lib/network", () => ({
  useNetwork: () => ({ network: "all", setNetwork: vi.fn() }),
  networkFilter: (network: string) => (network === "all" ? undefined : network),
}));

vi.mock("@/lib/api", () => ({
  searchContracts: (...args: unknown[]) => searchContracts(...args),
  resolveLabel: (...args: unknown[]) => resolveLabel(...args),
  getInvocationTrace: (...args: unknown[]) => getInvocationTrace(...args),
  ApiError: class ApiError extends Error {
    constructor(
      public status: number,
      message: string
    ) {
      super(message);
      this.name = "ApiError";
    }
  },
}));

import SearchPage from "@/app/(app)/search/page";

const CONTRACT_ID = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC";
const TX_HASH = "a".repeat(64);

function renderWithQuery(query: string) {
  for (const key of Array.from(searchParams.keys())) searchParams.delete(key);
  if (query) searchParams.set("q", query);
  return render(<SearchPage />);
}

describe("/search page", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("lists matching contracts and links to the detail page", async () => {
    searchContracts.mockResolvedValue({
      items: [
        {
          id: CONTRACT_ID,
          network: "testnet",
          label: "Aqua Token",
          status: "verified",
          wasm_hash: null,
          created_at_ledger: 1,
          backfill_complete_at: null,
          added_at: "2026-01-01T00:00:00Z",
          tags: [],
        },
      ],
    });
    const { ApiError } = await import("@/lib/api");
    resolveLabel.mockRejectedValue(new ApiError(404, "label not found"));

    renderWithQuery("aqua");

    const label = await screen.findByText("Aqua Token");
    expect(label.closest("a")?.getAttribute("href")).toBe(
      `/contracts/${CONTRACT_ID}`
    );
    expect(searchContracts).toHaveBeenCalledWith("aqua", {
      limit: 20,
      network: undefined,
    });
  });

  it("surfaces a search failure instead of an empty state", async () => {
    const { ApiError } = await import("@/lib/api");
    searchContracts.mockRejectedValue(new ApiError(503, "search is down"));
    resolveLabel.mockRejectedValue(new ApiError(404, "label not found"));

    renderWithQuery("aqua");

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "search is down"
    );
  });

  it("looks up a transaction hash", async () => {
    searchContracts.mockResolvedValue({ items: [] });
    const { ApiError } = await import("@/lib/api");
    resolveLabel.mockRejectedValue(new ApiError(404, "label not found"));
    getInvocationTrace.mockResolvedValue({
      tx_hash: TX_HASH,
      status: "success",
      network: "testnet",
      ledger: 42,
      root: {
        span_id: "root",
        cpu: 1,
        mem: 1,
        fee_share: 1,
        depth: 0,
        children: [],
      },
      edge_count: 2,
      has_edges: true,
      truncated: false,
    });

    renderWithQuery(TX_HASH);

    await waitFor(() =>
      expect(getInvocationTrace).toHaveBeenCalledWith(TX_HASH)
    );
    expect(await screen.findByText(/Ledger 42/)).toBeDefined();
  });

  it("prompts for a query when none is provided", () => {
    searchContracts.mockResolvedValue({ items: [] });

    renderWithQuery("");

    expect(screen.getByText(/Enter a query to search contracts/)).toBeDefined();
    expect(searchContracts).not.toHaveBeenCalled();
  });
});
