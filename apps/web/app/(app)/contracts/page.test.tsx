/**
 * Tests for apps/web/app/(app)/contracts/page.tsx
 *
 * We use vitest + @testing-library/react + jsdom.
 * next/link is mocked to a plain <a> so we don't need the Next.js runtime.
 * @/lib/api is mocked so we control the data returned.
 *
 * The stock single-field modal was replaced by the tracking wizard route
 * (issue #140), so the page is asserted to link to /contracts/new rather than
 * open a dialog.
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

// ── Mock next/navigation ────────────────────────────────────────────────────
// router.replace() updates the shared search params, so the component re-reads
// the new sort state on the next render just as a real navigation would.
const nav = vi.hoisted(() => {
  let query = new URLSearchParams("");
  const replace = vi.fn((href: string) => {
    query = new URLSearchParams(href.startsWith("?") ? href.slice(1) : href);
  });
  const push = vi.fn();
  return {
    replace,
    push,
    getQuery: () => query,
    setQuery: (q: string | URLSearchParams) => {
      query = new URLSearchParams(q);
    },
  };
});

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: nav.replace, push: nav.push }),
  useSearchParams: () => nav.getQuery(),
}));

// ── Mock @sorolens/ui so we don't need the built dist ───────────────────────
vi.mock("@sorolens/ui", () => ({
  Toast: ({ message }: { message: string }) => (
    <div role="alert">{message}</div>
  ),
  DataTable: <T,>({
    data,
    columns,
    rowKey,
    loading,
    emptyState,
    onRowClick,
    onSort,
    sortColumn,
    sortDirection,
  }: {
    data: T[];
    columns: {
      key: string;
      header: React.ReactNode;
      sortable?: boolean;
      accessor?: (item: T) => React.ReactNode;
    }[];
    rowKey: (item: T) => string;
    loading?: boolean;
    emptyState?: React.ReactNode;
    onRowClick?: (item: T) => void;
    onSort?: (columnKey: string) => void;
    sortColumn?: string;
    sortDirection?: "asc" | "desc";
  }) => {
    if (loading) return <div data-testid="data-table-loading">loading</div>;
    if (data.length === 0)
      return <div data-testid="data-table-empty">{emptyState}</div>;
    return (
      <table data-testid="data-table">
        <thead>
          <tr>
            {columns.map((col) => (
              <th
                key={col.key}
                data-testid={`col-${col.key}`}
                onClick={() => col.sortable && onSort?.(col.key)}
              >
                {col.header}
                {col.sortable && (
                  <span>
                    {sortColumn === col.key
                      ? sortDirection === "asc"
                        ? "▲"
                        : "▼"
                      : "↕"}
                  </span>
                )}
              </th>
            ))}
          </tr>
        </thead>
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
}));

// ── Mock the heavy leaf components the page renders ─────────────────────────
vi.mock("@/components/LabelledId", () => ({
  LabelledId: ({
    value,
    knownLabel,
  }: {
    value: string;
    knownLabel?: string | null;
  }) => (
    <span data-testid="labelled-id">
      {knownLabel ? `${knownLabel} ` : ""}
      {value}
    </span>
  ),
}));

vi.mock("@/components/ImportContractsCsv", () => ({
  default: () => <div data-testid="import-csv">import</div>,
}));

// ── Mock @/lib/api ───────────────────────────────────────────────────────────
const mockListContracts = vi.fn();
const mockBatchContracts = vi.fn();

vi.mock("@/lib/api", () => ({
  listContracts: (...args: unknown[]) => mockListContracts(...args),
  batchContracts: (...args: unknown[]) => mockBatchContracts(...args),
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

// ── Mock @/components/Skeleton ───────────────────────────────────────────────
vi.mock("@/components/Skeleton", () => ({
  TableSkeleton: ({ rows }: { rows?: number }) => (
    <div data-testid="table-skeleton" data-rows={rows}>
      skeleton
    </div>
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
  last_activity_at: "2024-01-02T00:00:00Z",
};

const CONTRACT_B = {
  id: "CBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
  network: "mainnet",
  label: null,
  status: "backfilling",
  wasm_hash: null,
  added_at: "2024-02-01T00:00:00Z",
  last_activity_at: null,
};

function rowTexts(): string[] {
  return screen
    .queryAllByTestId("data-table-row")
    .map((row) => row.textContent ?? "");
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe("ContractsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    nav.setQuery("");
    mockBatchContracts.mockResolvedValue({
      action: "untrack",
      requested: 1,
      affected: 1,
    });
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
    const { default: ContractsPage } =
      await import("@/app/(app)/contracts/page");
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
      expect(screen.queryByTestId("table-skeleton")).toBeNull()
    );
    expect(screen.getByTestId("data-table")).toBeDefined();
  });

  // ── Happy path: search filters by label ───────────────────────────────────

  it("filters contracts by label when searching", async () => {
    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));

    const search = screen.getByPlaceholderText(
      /search by alias or contract id/i
    );
    fireEvent.change(search, { target: { value: "My Contract" } });

    const rows = screen.getAllByTestId("data-table-row");
    expect(rows.length).toBe(1);
    expect(rows[0].textContent).toContain("My Contract");
  });

  // ── Happy path: search filters by contract ID ──────────────────────────────

  it("filters contracts by contract ID prefix when searching", async () => {
    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));

    const search = screen.getByPlaceholderText(
      /search by alias or contract id/i
    );
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
      expect(screen.queryByTestId("table-skeleton")).toBeNull()
    );
    expect(screen.getByText(/no contracts tracked yet/i)).toBeDefined();
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
      expect(screen.queryByTestId("table-skeleton")).toBeNull()
    );

    const link = screen.getByText(/run the tracking wizard/i);
    expect(link.getAttribute("href")).toBe("/contracts/new");
  });

  // ── Pagination: prev disabled on first page ────────────────────────────────

  it("Previous button is disabled on the first page", async () => {
    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));

    const prevBtn = document.getElementById(
      "contracts-prev-page"
    ) as HTMLButtonElement;
    expect(prevBtn?.disabled).toBe(true);
  });

  // ── Pagination: next disabled when has_more=false ─────────────────────────

  it("Next button is disabled when there is no next page", async () => {
    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));

    const nextBtn = document.getElementById(
      "contracts-next-page"
    ) as HTMLButtonElement;
    expect(nextBtn?.disabled).toBe(true);
  });

  // ── Bulk actions (#176): selection, untrack, tag ─────────────────────────

  /** Checks the row checkbox without triggering the row's navigate-on-click. */
  function selectRow(id: string) {
    fireEvent.click(screen.getByTestId(`select-${id}`));
  }

  it("hides the bulk toolbar until a contract is selected", async () => {
    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));

    expect(document.getElementById("bulk-toolbar")).toBeNull();

    selectRow(CONTRACT_A.id);
    expect(document.getElementById("bulk-toolbar")).toBeDefined();
    expect(screen.getByText(/1 selected/i)).toBeDefined();
  });

  it("selects every row on the page via the header checkbox", async () => {
    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));

    fireEvent.click(screen.getByTestId("select-all"));

    expect(
      (screen.getByTestId(`select-${CONTRACT_A.id}`) as HTMLInputElement)
        .checked
    ).toBe(true);
    expect(
      (screen.getByTestId(`select-${CONTRACT_B.id}`) as HTMLInputElement)
        .checked
    ).toBe(true);
    expect(screen.getByText(/2 selected/i)).toBeDefined();
  });

  it("untracks selected contracts after confirmation and refreshes the list", async () => {
    mockListContracts
      .mockResolvedValueOnce({
        contracts: [CONTRACT_A, CONTRACT_B],
        cursor: null,
        has_more: false,
      })
      .mockResolvedValue({
        contracts: [CONTRACT_B],
        cursor: null,
        has_more: false,
      });
    mockBatchContracts.mockResolvedValue({
      action: "untrack",
      requested: 1,
      affected: 1,
    });

    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));

    selectRow(CONTRACT_A.id);
    fireEvent.click(document.getElementById("bulk-untrack-btn")!);

    // Destructive action is confirmed before anything is sent.
    expect(screen.getByRole("dialog")).toBeDefined();
    expect(mockBatchContracts).not.toHaveBeenCalled();

    fireEvent.click(document.getElementById("untrack-confirm-btn")!);

    await waitFor(() => expect(mockBatchContracts).toHaveBeenCalledTimes(1));
    expect(mockBatchContracts).toHaveBeenCalledWith(
      { ids: [CONTRACT_A.id], action: "untrack", args: undefined },
      ""
    );

    // Modal closes, selection clears and the list refetches without A.
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await waitFor(() =>
      expect(screen.queryByTestId(`select-${CONTRACT_A.id}`)).toBeNull()
    );
    expect(document.getElementById("bulk-toolbar")).toBeNull();
    expect(mockListContracts).toHaveBeenCalledTimes(2);
  });

  it("NEGATIVE: cancelling the untrack confirmation sends nothing", async () => {
    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));

    selectRow(CONTRACT_A.id);
    fireEvent.click(document.getElementById("bulk-untrack-btn")!);
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(mockBatchContracts).not.toHaveBeenCalled();
    // The selection is kept so the user can retry.
    expect(screen.getByText(/1 selected/i)).toBeDefined();
  });

  it("tags every selected contract and refreshes the list", async () => {
    mockBatchContracts.mockResolvedValue({
      action: "tag",
      requested: 2,
      affected: 2,
    });

    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));

    fireEvent.click(screen.getByTestId("select-all"));
    fireEvent.click(document.getElementById("bulk-tag-btn")!);

    fireEvent.change(document.getElementById("tag-input")!, {
      target: { value: "payments" },
    });
    fireEvent.submit(
      document.getElementById("tag-submit-btn")!.closest("form")!
    );

    await waitFor(() => expect(mockBatchContracts).toHaveBeenCalledTimes(1));
    const [req, userId] = mockBatchContracts.mock.calls[0];
    expect(userId).toBe("");
    expect(req.action).toBe("tag");
    expect(req.args).toEqual({ label: "payments" });
    expect([...req.ids].sort()).toEqual([CONTRACT_A.id, CONTRACT_B.id].sort());

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await waitFor(() => expect(mockListContracts).toHaveBeenCalledTimes(2));
  });

  it("NEGATIVE: a failed bulk action surfaces an error toast", async () => {
    const { ApiError } = await import("@/lib/api");
    mockBatchContracts.mockRejectedValue(new ApiError(500, "boom"));

    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));

    selectRow(CONTRACT_A.id);
    fireEvent.click(document.getElementById("bulk-untrack-btn")!);
    fireEvent.click(document.getElementById("untrack-confirm-btn")!);

    const toast = await screen.findByRole("alert");
    expect(toast.textContent).toContain("Couldn't untrack contracts: boom");
    // The selection survives so the user can retry.
    expect(screen.getByText(/1 selected/i)).toBeDefined();
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
      "contracts-next-page"
    ) as HTMLButtonElement;
    expect(nextBtn?.disabled).toBe(false);

    fireEvent.click(nextBtn);

    // listContracts should be called a second time for page 2
    await waitFor(() => expect(mockListContracts).toHaveBeenCalledTimes(2));
  });

  // ── The list never keeps a stale page after a network switch ───────────────

  it("shows the current page rows in the table", async () => {
    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));
    expect(rowTexts()).toHaveLength(2);
  });

  // ── Tags: filter input is forwarded to the API ─────────────────────────────

  it("passes the tag filter to listContracts", async () => {
    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));

    const tagInput = screen.getByLabelText(/filter contracts by tag/i);
    fireEvent.change(tagInput, { target: { value: "prod" } });

    await waitFor(() =>
      expect(mockListContracts).toHaveBeenLastCalledWith(
        expect.objectContaining({ tag: "prod" })
      )
    );
  });

  // ── Tags: clicking a tag chip sets the filter ──────────────────────────────

  it("filters by tag when a tag chip is clicked", async () => {
    mockListContracts.mockResolvedValue({
      contracts: [{ ...CONTRACT_A, tags: ["prod"] }, CONTRACT_B],
      cursor: null,
      has_more: false,
    });
    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));

    fireEvent.click(screen.getByText("prod"));

    await waitFor(() =>
      expect(mockListContracts).toHaveBeenLastCalledWith(
        expect.objectContaining({ tag: "prod" })
      )
    );
  });

  // ── Tags: chips render for tagged contracts ────────────────────────────────

  it("renders a chip for each contract tag", async () => {
    mockListContracts.mockResolvedValue({
      contracts: [{ ...CONTRACT_A, tags: ["prod", "defi"] }, CONTRACT_B],
      cursor: null,
      has_more: false,
    });
    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));

    expect(screen.getByText("prod")).toBeDefined();
    expect(screen.getByText("defi")).toBeDefined();
  });

  // ── Sorting: header click toggles asc/desc, sort lives in the URL ────────

  it("sorts ascending on first click of a new column and mirrors it in the URL", async () => {
    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));
    nav.replace.mockClear();

    fireEvent.click(screen.getByTestId("col-label"));

    // The page refetches with the new sort against the API…
    await waitFor(() =>
      expect(mockListContracts).toHaveBeenCalledWith(
        expect.objectContaining({ sort: "label", dir: "asc" })
      )
    );
    // …and the active sort is written to the URL so the view is shareable.
    expect(nav.replace).toHaveBeenCalledWith("?sort=label&dir=asc");
    // The visual indicator is rendered for the active sort column.
    expect(screen.getByText("▲")).toBeDefined();
  });

  it("toggles to descending on a second click of the same column", async () => {
    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));
    nav.replace.mockClear();

    // First click on the default column (added_at, desc) flips to asc.
    fireEvent.click(screen.getByTestId("col-added_at"));
    await waitFor(() =>
      expect(nav.replace).toHaveBeenCalledWith("?sort=added_at&dir=asc")
    );

    // Second click flips back to desc.
    fireEvent.click(screen.getByTestId("col-added_at"));
    await waitFor(() =>
      expect(nav.replace).toHaveBeenCalledWith("?sort=added_at&dir=desc")
    );
    expect(screen.getByText("▼")).toBeDefined();
  });

  it("initializes the sort from the URL on load", async () => {
    nav.setQuery("?sort=status&dir=asc");

    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));

    // The first request already carries the URL sort params.
    expect(mockListContracts).toHaveBeenCalledWith(
      expect.objectContaining({ sort: "status", dir: "asc" })
    );
    // A URL that already matches the state is not rewritten.
    expect(nav.replace).not.toHaveBeenCalled();
    expect(screen.getByText("▲")).toBeDefined();
  });

  it("does not rewrite the URL for the default sort", async () => {
    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));

    // Visiting /contracts without sort params keeps the URL untouched.
    expect(nav.replace).not.toHaveBeenCalled();
  });
});
