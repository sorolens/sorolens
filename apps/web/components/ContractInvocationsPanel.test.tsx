import * as matchers from "@testing-library/jest-dom/matchers";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ContractInvocationsPanel } from "./ContractInvocationsPanel";
import type { Invocation } from "@/lib/types";

expect.extend(matchers);

const mockGetContractInvocations = vi.fn();

vi.mock("@/lib/api", () => ({
  getContractInvocations: (...args: unknown[]) =>
    mockGetContractInvocations(...args),
}));

vi.mock("@sorolens/ui", () => ({
  MonoId: ({ value }: { value: string }) => <span>{value}</span>,
  DataTable: ({
    data,
    columns,
  }: {
    data: Invocation[];
    columns: {
      key: string;
      header: string;
      accessor?: (item: Invocation) => React.ReactNode;
    }[];
  }) => (
    <table data-testid="data-table">
      <thead>
        <tr>
          {columns.map((col) => (
            <th key={col.key}>{col.header}</th>
          ))}
        </tr>
      </thead>
      <tbody>
        {data.map((inv, idx) => (
          <tr key={idx} data-testid="invocation-row">
            {columns.map((col) => (
              <td key={col.key}>
                {col.accessor
                  ? col.accessor(inv)
                  : String(
                      (inv as unknown as Record<string, unknown>)[col.key] ?? ""
                    )}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  ),
}));

vi.mock("@/components/Skeleton", () => ({
  TableSkeleton: () => <div data-testid="table-skeleton">Loading table...</div>,
}));

const mockInvocationA: Invocation = {
  tx_hash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
  contract_id: "CAVRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5",
  network: "testnet",
  ledger: 123456,
  ledger_closed_at: "2026-11-18T10:00:00Z",
  status: "success",
  function_name: "deposit",
  args_decoded: null,
  result_decoded: null,
  resource_fee_charged: 100,
  cpu_insn: 50000,
  mem_byte: 1024,
  ledger_read_byte: 200,
  ledger_write_byte: 300,
};

const mockInvocationB: Invocation = {
  ...mockInvocationA,
  tx_hash: "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210",
  function_name: "withdraw",
  status: "failed",
};

describe("ContractInvocationsPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetContractInvocations.mockResolvedValue({
      invocations: [mockInvocationA, mockInvocationB],
      next_cursor: "",
    });
  });

  afterEach(() => {
    cleanup();
  });

  it("renders invocations table with data", async () => {
    render(
      <ContractInvocationsPanel contractId={mockInvocationA.contract_id} />
    );

    expect(screen.getByTestId("table-skeleton")).toBeInTheDocument();

    await waitFor(() => {
      expect(
        screen.getByTestId("contract-invocations-panel")
      ).toBeInTheDocument();
    });

    expect(await screen.findByText("deposit")).toBeInTheDocument();
    expect(screen.getByText("withdraw")).toBeInTheDocument();
    expect(screen.getByText("success")).toBeInTheDocument();
    expect(screen.getByText("failed")).toBeInTheDocument();
  });

  it("filters by function name", async () => {
    render(
      <ContractInvocationsPanel contractId={mockInvocationA.contract_id} />
    );
    await waitFor(() =>
      expect(
        screen.getByTestId("invocation-function-filter")
      ).toBeInTheDocument()
    );

    const input = screen.getByTestId("invocation-function-filter");
    fireEvent.change(input, { target: { value: "deposit" } });

    await waitFor(() => {
      expect(mockGetContractInvocations).toHaveBeenCalledWith(
        mockInvocationA.contract_id,
        expect.objectContaining({ function_name: "deposit" })
      );
    });
  });

  it("filters by status", async () => {
    render(
      <ContractInvocationsPanel contractId={mockInvocationA.contract_id} />
    );
    await waitFor(() =>
      expect(screen.getByTestId("invocation-status-filter")).toBeInTheDocument()
    );

    const select = screen.getByTestId("invocation-status-filter");
    fireEvent.change(select, { target: { value: "failed" } });

    await waitFor(() => {
      expect(mockGetContractInvocations).toHaveBeenCalledWith(
        mockInvocationA.contract_id,
        expect.objectContaining({ status: "failed" })
      );
    });
  });

  it("supports load more pagination", async () => {
    mockGetContractInvocations
      .mockResolvedValueOnce({
        invocations: [mockInvocationA],
        next_cursor: "page-2-cursor",
      })
      .mockResolvedValueOnce({
        invocations: [mockInvocationB],
        next_cursor: "",
      });

    render(
      <ContractInvocationsPanel contractId={mockInvocationA.contract_id} />
    );

    const loadMoreBtn = await screen.findByTestId("load-more-invocations");
    expect(loadMoreBtn).toBeInTheDocument();

    fireEvent.click(loadMoreBtn);

    await waitFor(() => {
      expect(mockGetContractInvocations).toHaveBeenCalledWith(
        mockInvocationA.contract_id,
        expect.objectContaining({ cursor: "page-2-cursor" })
      );
    });
  });
});
