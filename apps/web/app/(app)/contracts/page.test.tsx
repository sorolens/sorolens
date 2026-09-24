/**
 * Tests for apps/web/app/(app)/contracts/page.tsx
 *
 * We use vitest + @testing-library/react + jsdom.
 * next/link is mocked to a plain <a> so we don't need the Next.js runtime.
 * @/lib/api is mocked so we control the data returned.
 */

import * as matchers from "@testing-library/jest-dom/matchers";
import {
  cleanup,
  render,
  screen,
  fireEvent,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// ── Mock next/link ──────────────────────────────────────────────────────────
// Props other than href (id, className, …) are forwarded, so tests can still
// target elements by id now that the track button is a Link.
vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    ...rest
  }: React.AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

// ── Mock @sorolens/ui so we don't need the built dist ───────────────────────
vi.mock("@sorolens/ui", () => ({
  DataTable: <T,>({
    data,
    columns,
    rowKey,
    loading,
    emptyState,
    onRowClick,
  }: {
    data: T[];
    columns: { key: string; header: string; accessor?: (item: T) => React.ReactNode }[];
    rowKey: (item: T) => string;
    loading?: boolean;
    emptyState?: React.ReactNode;
    onRowClick?: (item: T) => void;
  }) => {
    if (loading) return <div data-testid="data-table-loading">loading</div>;
    if (data.length === 0) return <div data-testid="data-table-empty">{emptyState}</div>;
    return (
      <table data-testid="data-table">
        <tbody>
          {data.map((item) => (
            <tr
              key={rowKey(item)}
              data-testid="data-table-row"
              onClick={() => onRowClick?.(item)}
            >
              {columns.map((col) => (
                <td key={col.key}>
                  {col.accessor
                    ? col.accessor(item)
                    : String((item as Record<string, unknown>)[col.key] ?? "")}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    );
  },
  MonoId: ({ value }: { value: string }) => (
    <span data-testid="mono-id">{value.slice(0, 8)}…{value.slice(-8)}</span>
  ),
}));

// ── Mock @/lib/api ───────────────────────────────────────────────────────────
const mockListContracts = vi.fn();
const mockTrackContract = vi.fn();

vi.mock("@/lib/api", () => ({
  listContracts: (...args: unknown[]) => mockListContracts(...args),
  trackContract: (...args: unknown[]) => mockTrackContract(...args),
  ApiError: class ApiError extends Error {
    constructor(
      public status: number,
      message: string,
    ) {
      super(message);
      this.name = "ApiError";
    }
  },
}));

// ── Mock @/components/Skeleton ───────────────────────────────────────────────
vi.mock("@/components/Skeleton", () => ({
  TableSkeleton: ({ rows }: { rows?: number }) => (
    <div data-testid="table-skeleton" data-rows={rows}>skeleton</div>
  ),
}));

expect.extend(matchers);

// ── Fixtures ─────────────────────────────────────────────────────────────────

const CONTRACT_A = {
  id: "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
  network: "testnet",
  label: "My Contract",
  status: "active",
  wasm_hash: null,
  added_at: "2024-01-01T00:00:00Z",
};

const CONTRACT_B = {
  id: "CBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
  network: "mainnet",
  label: null,
  status: "backfilling",
  wasm_hash: null,
  added_at: "2024-02-01T00:00:00Z",
};

const VALID_CONTRACT_ID =
  "CC3W4K5J6H7G8F9E0D1C2B3A4Z5Y6X7W8V9U0T1S2R3Q4P5O6N7M8L9K";

// ── Tests ─────────────────────────────────────────────────────────────────────

describe("ContractsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListContracts.mockResolvedValue({
      contracts: [CONTRACT_A, CONTRACT_B],
      cursor: null,
      has_more: false,
    });
  });

  afterEach(() => {
    cleanup();
  });

  // Helper: dynamic import so mocks are in place before module loads
  async function renderPage() {
    const { default: ContractsPage } = await import(
      "@/app/(app)/contracts/page"
    );
    return render(<ContractsPage />);
  }

  // ── Happy path: renders heading ────────────────────────────────────────────

  it("renders the Contracts heading", async () => {
    await renderPage();
    expect(screen.getByRole("heading", { name: /contracts/i })).toBeDefined();
  });

  // ── Happy path: shows loading skeleton initially ───────────────────────────

  it("shows skeleton while loading", async () => {
    // Never resolves during this test
    mockListContracts.mockReturnValue(new Promise(() => {}));
    await renderPage();
    expect(screen.getByTestId("table-skeleton")).toBeDefined();
  });

  // ── Happy path: renders data table after load ─────────────────────────────

  it("renders the DataTable with contracts after loading", async () => {
    await renderPage();
    await waitFor(() =>
      expect(screen.queryByTestId("table-skeleton")).toBeNull(),
    );
    expect(screen.getByTestId("data-table")).toBeDefined();
  });

  // ── Happy path: search filters by label ───────────────────────────────────

  it("filters contracts by label when searching", async () => {
    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));

    const search = screen.getByPlaceholderText(
      /search by alias or contract id/i,
    );
    fireEvent.change(search, { target: { value: "My Contract" } });

    // After filtering, only CONTRACT_A (label='My Contract') should be present
    // DataTable renders rows; CONTRACT_B has null label, so it should be gone
    const rows = screen.getAllByTestId("data-table-row");
    expect(rows.length).toBe(1);
    expect(rows[0].textContent).toContain("My Contract");
  });

  // ── Happy path: search filters by contract ID ──────────────────────────────

  it("filters contracts by contract ID prefix when searching", async () => {
    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));

    const search = screen.getByPlaceholderText(
      /search by alias or contract id/i,
    );
    // CONTRACT_A id starts with CAAAA, CONTRACT_B with CBBBB
    fireEvent.change(search, { target: { value: "CBBBB" } });

    const rows = screen.getAllByTestId("data-table-row");
    expect(rows.length).toBe(1);
  });

  // ── Happy path: empty state ────────────────────────────────────────────────

  it("shows empty state when no contracts are tracked", async () => {
    mockListContracts.mockResolvedValue({
      contracts: [],
      cursor: null,
      has_more: false,
    });
    await renderPage();
    await waitFor(() =>
      expect(screen.queryByTestId("table-skeleton")).toBeNull(),
    );
    expect(
      screen.getByText(/no contracts tracked yet/i),
    ).toBeDefined();
  });

  // ── Track contract entry point (wizard, issue #140) ────────────────────────

  it("links the Track contract button to the wizard route", async () => {
    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));

    const btn = document.getElementById("track-contract-btn");
    expect(btn).not.toBeNull();
    expect(btn?.getAttribute("href")).toBe("/contracts/new");
    // The single-field modal is gone: nothing is a dialog any more.
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("links the empty-state prompt to the wizard route", async () => {
    mockListContracts.mockResolvedValue({
      contracts: [],
      cursor: null,
      has_more: false,
    });
    await renderPage();
    await waitFor(() =>
      expect(screen.queryByTestId("table-skeleton")).toBeNull(),
    );

    const link = screen.getByText(/run the tracking wizard/i);
    expect(link.getAttribute("href")).toBe("/contracts/new");
  });

  // ── Pagination: prev disabled on first page ────────────────────────────────

  it("Previous button is disabled on the first page", async () => {
    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));

    const prevBtn = document.getElementById(
      "contracts-prev-page",
    ) as HTMLButtonElement;
    expect(prevBtn?.disabled).toBe(true);
  });

  // ── Pagination: next disabled when has_more=false ─────────────────────────

  it("Next button is disabled when there is no next page", async () => {
    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));

    const nextBtn = document.getElementById(
      "contracts-next-page",
    ) as HTMLButtonElement;
    expect(nextBtn?.disabled).toBe(true);
  });

  // ── Pagination: next enabled and advances when has_more=true ─────────────

  it("Next button is enabled and triggers next page fetch when has_more=true", async () => {
    mockListContracts
      .mockResolvedValueOnce({
        contracts: [CONTRACT_A, CONTRACT_B],
        cursor: CONTRACT_B.id,
        has_more: true,
      })
      .mockResolvedValueOnce({
        contracts: [],
        cursor: null,
        has_more: false,
      });

    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));

    const nextBtn = document.getElementById(
      "contracts-next-page",
    ) as HTMLButtonElement;
    expect(nextBtn?.disabled).toBe(false);

    fireEvent.click(nextBtn);

    // listContracts should be called a second time for page 2
    await waitFor(() =>
      expect(mockListContracts).toHaveBeenCalledTimes(2),
    );
  });
});
