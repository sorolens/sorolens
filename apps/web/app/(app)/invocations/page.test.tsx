/**
 * Tests for apps/web/app/(app)/invocations/page.tsx
 *
 * vitest + @testing-library/react + jsdom. @sorolens/ui is mocked with a
 * DataTable whose headers are clickable so the sort handlers can be exercised,
 * and @/lib/api is mocked so the tests control the returned page.
 */

import * as matchers from "@testing-library/jest-dom/matchers";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// ── Mock @sorolens/ui (DataTable renders sortable headers) ──────────────────
vi.mock("@sorolens/ui", () => ({
  DataTable: <T,>({
    data,
    columns,
    rowKey,
    emptyState,
    sortColumn,
    sortDirection,
    onSort,
    onRowClick,
  }: {
    data: T[];
    columns: {
      key: string;
      header: string;
      sortable?: boolean;
      accessor?: (item: T) => React.ReactNode;
    }[];
    rowKey: (item: T) => string;
    emptyState?: React.ReactNode;
    sortColumn?: string;
    sortDirection?: "asc" | "desc";
    onSort?: (columnKey: string) => void;
    onRowClick?: (item: T) => void;
  }) => {
    if (data.length === 0) {
      return <div data-testid="data-table-empty">{emptyState}</div>;
    }
    return (
      <table data-testid="data-table">
        <thead>
          <tr>
            {columns.map((col) => (
              <th key={col.key}>
                <button
                  type="button"
                  data-testid={`sort-${col.key}`}
                  onClick={() => col.sortable && onSort?.(col.key)}
                >
                  {col.header}
                  {sortColumn === col.key
                    ? sortDirection === "asc"
                      ? "▲"
                      : "▼"
                    : ""}
                </button>
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
                  {col.accessor ? col.accessor(item) : String((item as Record<string, unknown>)[col.key] ?? "")}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    );
  },
  MonoId: ({ value }: { value: string }) => (
    <span data-testid="mono-id">
      {value.slice(0, 8)}…{value.slice(-8)}
    </span>
  ),
}));

// ── Mock @/components/Skeleton ──────────────────────────────────────────────
vi.mock("@/components/Skeleton", () => ({
  TableSkeleton: ({ rows }: { rows?: number }) => (
    <div data-testid="table-skeleton" data-rows={rows}>
      skeleton
    </div>
  ),
}));

// ── Mock @/lib/api ──────────────────────────────────────────────────────────
const mockListInvocations = vi.fn();

vi.mock("@/lib/api", () => ({
  listInvocations: (...args: unknown[]) => mockListInvocations(...args),
}));

expect.extend(matchers);

// ── Fixtures ────────────────────────────────────────────────────────────────

function invocation(overrides: Record<string, unknown>) {
  return {
    tx_hash: "hash",
    contract_id: "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
    network: "testnet",
    ledger: 100,
    ledger_closed_at: "2026-09-01T12:00:00Z",
    status: "SUCCESS",
    function_name: "transfer",
    args_decoded: null,
    result_decoded: null,
    resource_fee_charged: 10,
    cpu_insn: 1000,
    mem_byte: 100,
    ledger_read_byte: 10,
    ledger_write_byte: 20,
    ...overrides,
  };
}

const INVOCATIONS = [
  invocation({
    tx_hash: "aa",
    ledger: 100,
    ledger_closed_at: "2026-09-01T12:00:00Z",
    function_name: "transfer",
    cpu_insn: 500,
    resource_fee_charged: 30,
  }),
  invocation({
    tx_hash: "bb",
    ledger: 200,
    ledger_closed_at: "2026-09-02T12:00:00Z",
    function_name: "mint",
    cpu_insn: 100,
    resource_fee_charged: 10,
  }),
  invocation({
    tx_hash: "cc",
    ledger: 300,
    ledger_closed_at: "2026-09-03T12:00:00Z",
    function_name: "burn",
    cpu_insn: 900,
    resource_fee_charged: 50,
  }),
];

function rowTexts(): string[] {
  return screen
    .queryAllByTestId("data-table-row")
    .map((row) => row.textContent ?? "");
}

async function renderPage() {
  const { default: InvocationsPage } = await import(
    "@/app/(app)/invocations/page"
  );
  return render(<InvocationsPage />);
}

// ── Tests ───────────────────────────────────────────────────────────────────

describe("InvocationsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListInvocations.mockResolvedValue({
      invocations: INVOCATIONS,
      next_cursor: null,
    });
  });

  afterEach(() => {
    cleanup();
  });

  it("renders the heading and a row per invocation", async () => {
    await renderPage();

    expect(
      screen.getByRole("heading", { name: /invocations/i }),
    ).toBeDefined();

    await waitFor(() => {
      expect(screen.getAllByTestId("data-table-row")).toHaveLength(3);
    });
  });

  it("requests the first page with the configured page size", async () => {
    await renderPage();

    await waitFor(() => {
      expect(mockListInvocations).toHaveBeenCalledWith(
        expect.objectContaining({ cursor: undefined, limit: 20 }),
      );
    });
  });

  it("shows the empty state when the API returns no invocations", async () => {
    mockListInvocations.mockResolvedValue({ invocations: [], next_cursor: null });

    await renderPage();

    await waitFor(() => {
      expect(screen.getByText(/no invocations indexed yet/i)).toBeDefined();
    });
  });

  it("shows a distinct empty state when filters are active", async () => {
    mockListInvocations.mockResolvedValue({ invocations: [], next_cursor: null });

    await renderPage();

    fireEvent.change(document.getElementById("invocations-contract-filter")!, {
      target: { value: "CBBB" },
    });
    fireEvent.submit(
      document.getElementById("invocations-apply-filter")!.closest("form")!,
    );

    await waitFor(() => {
      expect(mockListInvocations).toHaveBeenLastCalledWith(
        expect.objectContaining({ contract_id: "CBBB" }),
      );
    });
    await waitFor(() => {
      expect(
        screen.getByText(/no invocations match your filters/i),
      ).toBeDefined();
    });
  });

  it("applies the function name and date range filters", async () => {
    await renderPage();

    fireEvent.change(document.getElementById("invocations-fn-filter")!, {
      target: { value: "mint" },
    });
    fireEvent.change(document.getElementById("invocations-since-filter")!, {
      target: { value: "2026-09-01" },
    });
    fireEvent.change(document.getElementById("invocations-until-filter")!, {
      target: { value: "2026-09-30" },
    });
    fireEvent.submit(
      document.getElementById("invocations-apply-filter")!.closest("form")!,
    );

    await waitFor(() => {
      expect(mockListInvocations).toHaveBeenLastCalledWith(
        expect.objectContaining({
          fn: "mint",
          since: "2026-09-01",
          until: "2026-09-30",
        }),
      );
    });
  });

  it("clears filters and refetches the unfiltered first page", async () => {
    await renderPage();

    fireEvent.change(document.getElementById("invocations-fn-filter")!, {
      target: { value: "mint" },
    });
    fireEvent.submit(
      document.getElementById("invocations-apply-filter")!.closest("form")!,
    );
    await waitFor(() =>
      expect(mockListInvocations).toHaveBeenLastCalledWith(
        expect.objectContaining({ fn: "mint" }),
      ),
    );

    fireEvent.click(document.getElementById("invocations-clear-filter")!);

    await waitFor(() => {
      expect(mockListInvocations).toHaveBeenLastCalledWith(
        expect.objectContaining({ fn: undefined, contract_id: undefined }),
      );
    });
  });

  it("sorts by CPU instructions ascending then descending", async () => {
    await renderPage();
    await waitFor(() =>
      expect(screen.getAllByTestId("data-table-row")).toHaveLength(3),
    );

    // Initially newest first (mint is oldest of the three? no: burn is newest).
    expect(rowTexts()[0]).toContain("burn");

    // Click CPU header -> ascending by cpu_insn: bb(100), aa(500), cc(900).
    fireEvent.click(screen.getByTestId("sort-cpu_insn"));
    expect(rowTexts()[0]).toContain("mint");
    expect(rowTexts()[2]).toContain("burn");

    // Click again -> descending: cc(900), aa(500), bb(100).
    fireEvent.click(screen.getByTestId("sort-cpu_insn"));
    expect(rowTexts()[0]).toContain("burn");
    expect(rowTexts()[2]).toContain("mint");
  });

  it("sorts by fee charged", async () => {
    await renderPage();
    await waitFor(() =>
      expect(screen.getAllByTestId("data-table-row")).toHaveLength(3),
    );

    // Ascending fee: bb(10), aa(30), cc(50).
    fireEvent.click(screen.getByTestId("sort-resource_fee_charged"));
    expect(rowTexts()[0]).toContain("mint");
    expect(rowTexts()[2]).toContain("burn");
  });

  it("paginates forward and back with the cursor", async () => {
    mockListInvocations
      .mockResolvedValueOnce({ invocations: [INVOCATIONS[2]], next_cursor: "200:bb" })
      .mockResolvedValueOnce({ invocations: [INVOCATIONS[1]], next_cursor: null })
      .mockResolvedValue({ invocations: [INVOCATIONS[2]], next_cursor: "200:bb" });

    await renderPage();
    await waitFor(() =>
      expect(screen.getAllByTestId("data-table-row")).toHaveLength(1),
    );

    const next = document.getElementById("invocations-next-page") as HTMLButtonElement;
    expect(next.disabled).toBe(false);

    fireEvent.click(next);
    await waitFor(() => {
      expect(mockListInvocations).toHaveBeenLastCalledWith(
        expect.objectContaining({ cursor: "200:bb" }),
      );
    });

    const prev = document.getElementById("invocations-prev-page") as HTMLButtonElement;
    expect(prev.disabled).toBe(false);
    fireEvent.click(prev);
    await waitFor(() => {
      expect(mockListInvocations).toHaveBeenLastCalledWith(
        expect.objectContaining({ cursor: undefined }),
      );
    });
  });

  it("disables Next on the last page", async () => {
    await renderPage();
    await waitFor(() =>
      expect(screen.getAllByTestId("data-table-row")).toHaveLength(3),
    );

    const next = document.getElementById("invocations-next-page") as HTMLButtonElement;
    expect(next.disabled).toBe(true);
  });
});
