/**
 * Tests for apps/web/app/(app)/contracts/page.tsx
 *
 * We use vitest + @testing-library/react + jsdom.
 * next/link is mocked to a plain <a> so we don't need the Next.js runtime.
 * @/lib/api is mocked so we control the data returned.
 */

import * as matchers from "@testing-library/jest-dom/matchers";
import {
  act,
  cleanup,
  render,
  screen,
  fireEvent,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// ── Mock next/link ──────────────────────────────────────────────────────────
vi.mock("next/link", () => ({
  default: ({
    children,
    href,
  }: {
    children: React.ReactNode;
    href: string;
  }) => <a href={href}>{children}</a>,
}));

// ── Mock @sorolens/ui so we don't need the built dist ───────────────────────
// Toast is the real component so the tests assert what users actually get.
vi.mock("@sorolens/ui", async (importOriginal) => ({
  Toast: (await importOriginal<typeof import("@sorolens/ui")>()).Toast,
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

const NEW_CONTRACT = {
  id: VALID_CONTRACT_ID,
  label: "New Contract",
  status: "backfilling",
  network: "testnet",
  wasm_hash: null,
  added_at: "2024-03-01T00:00:00Z",
};

// ── Helpers ──────────────────────────────────────────────────────────────────

/** A promise the test settles by hand, to observe the UI mid-request. */
function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function rowTexts(): string[] {
  return screen
    .queryAllByTestId("data-table-row")
    .map((row) => row.textContent ?? "");
}

/** Opens the Track modal, fills the form and submits it. */
function submitTrack(contractId: string, label?: string) {
  fireEvent.click(document.getElementById("track-contract-btn")!);
  fireEvent.change(screen.getByLabelText(/contract id/i), {
    target: { value: contractId },
  });
  if (label) {
    fireEvent.change(screen.getByLabelText(/alias/i), {
      target: { value: label },
    });
  }
  const submitEl = document.getElementById(
    "track-modal-submit",
  ) as HTMLButtonElement;
  fireEvent.submit(submitEl.closest("form")!);
}

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

  // ── Happy path: track contract modal open/close ────────────────────────────

  it("opens Track contract modal when the button is clicked", async () => {
    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));

    fireEvent.click(document.getElementById("track-contract-btn")!);
    expect(screen.getByRole("dialog")).toBeDefined();
    expect(screen.getByLabelText(/contract id/i)).toBeDefined();
  });

  it("closes the modal when Escape key is pressed", async () => {
    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));

    fireEvent.click(document.getElementById("track-contract-btn")!);
    expect(screen.getByRole("dialog")).toBeDefined();

    fireEvent.keyDown(window, { key: "Escape" });
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).toBeNull(),
    );
  });

  // ── Validation: invalid contract ID is rejected ────────────────────────────

  it("shows validation error for invalid contract ID format", async () => {
    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));

    fireEvent.click(document.getElementById("track-contract-btn")!);

    const input = screen.getByLabelText(/contract id/i);
    fireEvent.change(input, { target: { value: "NOT_A_VALID_ID" } });

    expect(
      screen.getByText(/contract id must be 56 characters/i),
    ).toBeDefined();
  });

  it("NEGATIVE: submit button is disabled when contract ID is invalid", async () => {
    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));

    fireEvent.click(document.getElementById("track-contract-btn")!);

    const input = screen.getByLabelText(/contract id/i);
    fireEvent.change(input, { target: { value: "BAD" } });

    const submitEl = document.getElementById("track-modal-submit") as HTMLButtonElement;
    expect(submitEl?.disabled).toBe(true);
  });

  // ── Happy path: successful track contract submission ───────────────────────

  it("calls trackContract and refreshes on valid submission", async () => {
    mockTrackContract.mockResolvedValue(NEW_CONTRACT);

    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));

    submitTrack(VALID_CONTRACT_ID);

    await waitFor(() =>
      expect(mockTrackContract).toHaveBeenCalledWith(
        {
          id: VALID_CONTRACT_ID,
          label: undefined,
        },
        "",
      ),
    );
    // Modal closes on submit
    expect(screen.queryByRole("dialog")).toBeNull();
    // listContracts was called again to refresh
    await waitFor(() => expect(mockListContracts).toHaveBeenCalledTimes(2));
  });

  // ── Optimistic track ───────────────────────────────────────────────────────

  it("prepends the new contract immediately, before the API responds", async () => {
    const track = deferred<typeof NEW_CONTRACT>();
    mockTrackContract.mockReturnValue(track.promise);

    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));
    const before = rowTexts();

    submitTrack(VALID_CONTRACT_ID, "New Contract");

    // The request is still pending, yet the row is already at the top.
    expect(mockTrackContract).toHaveBeenCalledTimes(1);
    const rows = rowTexts();
    expect(rows).toHaveLength(3);
    expect(rows[0]).toContain(VALID_CONTRACT_ID.slice(0, 8));
    expect(rows[0]).toContain("New Contract");
    expect(rows[0]).toContain("pending");
    // Existing contracts are untouched and keep their order.
    expect(rows.slice(1)).toEqual(before);
    // No refetch yet, no detail link for the unconfirmed row, and no second
    // submission while this one is in flight.
    expect(mockListContracts).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("link", { name: "New Contract" })).toBeNull();
    expect(screen.getByRole("link", { name: "My Contract" })).toBeDefined();
    expect(
      (document.getElementById("track-contract-btn") as HTMLButtonElement)
        .disabled,
    ).toBe(true);

    await act(async () => track.resolve(NEW_CONTRACT));
  });

  it("reconciles via refetch on success without duplicating the new contract", async () => {
    const track = deferred<typeof NEW_CONTRACT>();
    mockTrackContract.mockReturnValue(track.promise);

    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));
    const before = rowTexts();

    // The server now returns the new contract alongside the existing ones.
    mockListContracts.mockResolvedValue({
      contracts: [NEW_CONTRACT, CONTRACT_A, CONTRACT_B],
      cursor: null,
      has_more: false,
    });
    submitTrack(VALID_CONTRACT_ID, "New Contract");
    expect(rowTexts()).toHaveLength(3);

    await act(async () => track.resolve(NEW_CONTRACT));

    await waitFor(() => expect(mockListContracts).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(rowTexts()[0]).toContain("backfilling"));
    const rows = rowTexts();
    expect(rows).toHaveLength(3);
    expect(
      rows.filter((r) => r.includes(VALID_CONTRACT_ID.slice(0, 8))),
    ).toHaveLength(1);
    expect(rows.some((r) => r.includes("pending"))).toBe(false);
    expect(rows.slice(1)).toEqual(before);
    expect(screen.getByRole("link", { name: "New Contract" })).toBeDefined();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("NEGATIVE: restores the previous list and shows a toast when the API fails", async () => {
    const { ApiError } = await import("@/lib/api");
    const track = deferred<typeof NEW_CONTRACT>();
    mockTrackContract.mockReturnValue(track.promise);

    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));
    const before = rowTexts();

    submitTrack(VALID_CONTRACT_ID, "New Contract");
    expect(rowTexts()).toHaveLength(3);

    await act(async () =>
      track.reject(new ApiError(409, "Contract already tracked")),
    );

    const toast = await screen.findByRole("alert");
    expect(toast.textContent).toContain(
      "Couldn't track contract: Contract already tracked",
    );
    expect(rowTexts()).toEqual(before);
    // Rolled back locally, not refetched.
    expect(mockListContracts).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(
      (document.getElementById("track-contract-btn") as HTMLButtonElement)
        .disabled,
    ).toBe(false);
  });

  it("NEGATIVE: falls back to a generic toast message for non-API errors", async () => {
    mockTrackContract.mockRejectedValue(new TypeError("Failed to fetch"));

    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));
    const before = rowTexts();

    submitTrack(VALID_CONTRACT_ID);

    const toast = await screen.findByRole("alert");
    expect(toast.textContent).toContain(
      "Couldn't track contract. Please try again.",
    );
    expect(rowTexts()).toEqual(before);
  });

  it("NEGATIVE: invalid input is still rejected client-side, with no optimistic row or toast", async () => {
    await renderPage();
    await waitFor(() => screen.getByTestId("data-table"));
    const before = rowTexts();

    submitTrack("NOT_A_VALID_ID");

    expect(mockTrackContract).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog")).toBeDefined();
    // The only alert is the inline validation message, not a toast.
    expect(screen.getByRole("alert").textContent).toMatch(
      /contract id must be 56 characters/i,
    );
    expect(screen.queryByText(/couldn't track contract/i)).toBeNull();
    expect(rowTexts()).toEqual(before);
  });

  it("ignores a stale list response that lands after the optimistic add", async () => {
    const { ApiError } = await import("@/lib/api");
    const staleLoad = deferred<unknown>();
    mockListContracts.mockReturnValueOnce(staleLoad.promise);
    const track = deferred<typeof NEW_CONTRACT>();
    mockTrackContract.mockReturnValue(track.promise);

    await renderPage();
    expect(screen.getByTestId("table-skeleton")).toBeDefined();

    submitTrack(VALID_CONTRACT_ID, "New Contract");
    expect(rowTexts()).toHaveLength(1);

    // The load that started before the submit must not wipe the new row.
    await act(async () =>
      staleLoad.resolve({
        contracts: [CONTRACT_A, CONTRACT_B],
        cursor: null,
        has_more: false,
      }),
    );
    expect(rowTexts()).toHaveLength(1);
    expect(rowTexts()[0]).toContain("New Contract");

    // On failure the interrupted page load is fetched again.
    await act(async () => track.reject(new ApiError(500, "boom")));
    await screen.findByRole("alert");
    await waitFor(() => expect(mockListContracts).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(rowTexts()).toHaveLength(2));
    expect(screen.queryByText("New Contract")).toBeNull();
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
