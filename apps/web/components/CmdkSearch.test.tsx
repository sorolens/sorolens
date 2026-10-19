import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

const { push, searchContracts } = vi.hoisted(() => ({
  push: vi.fn(),
  searchContracts: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push,
    replace: vi.fn(),
    back: vi.fn(),
    prefetch: vi.fn(),
  }),
}));

vi.mock("@/components/ThemeProvider", () => ({
  useTheme: () => ({ theme: "light", setTheme: vi.fn() }),
}));

vi.mock("@/lib/network", () => ({
  useNetwork: () => ({ network: "all", setNetwork: vi.fn() }),
  networkFilter: (network: string) => (network === "all" ? undefined : network),
}));

vi.mock("@/lib/api", () => ({
  searchContracts: (...args: unknown[]) => searchContracts(...args),
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

import { CmdkSearch } from "@/components/CmdkSearch";

const CONTRACT_ID = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC";

function openWithKeyboard() {
  fireEvent.keyDown(document, { key: "k", ctrlKey: true });
}

describe("CmdkSearch", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("exposes desktop and mobile triggers", () => {
    render(<CmdkSearch />);
    expect(
      screen.getAllByRole("button", { name: "Search (Ctrl+K)" })
    ).toHaveLength(2);
  });

  it("opens on Ctrl+K, fetches results and navigates on select", async () => {
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

    render(<CmdkSearch />);
    openWithKeyboard();

    const input = await screen.findByPlaceholderText(
      "Search contracts by ID or label..."
    );
    fireEvent.change(input, { target: { value: "aqua" } });

    // use-debounce waits 300ms before firing the request, so allow slack.
    const result = await screen.findByText("Aqua Token", {}, { timeout: 3000 });
    expect(searchContracts).toHaveBeenCalledWith("aqua", {
      limit: 10,
      network: undefined,
    });

    fireEvent.click(result.closest("[cmdk-item]") as HTMLElement);
    await waitFor(() =>
      expect(push).toHaveBeenCalledWith(`/contracts/${CONTRACT_ID}`)
    );
  });

  it("shows an error message when the search request fails", async () => {
    const { ApiError } = await import("@/lib/api");
    searchContracts.mockRejectedValue(new ApiError(503, "search is down"));

    render(<CmdkSearch />);
    openWithKeyboard();

    const input = await screen.findByPlaceholderText(
      "Search contracts by ID or label..."
    );
    fireEvent.change(input, { target: { value: "aqua" } });

    expect(
      await screen.findByText("search is down", {}, { timeout: 3000 })
    ).toBeDefined();
  });
});
