import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ContractPicker } from "./ContractPicker";
import type { MonitoredContract } from "@/lib/types";

const mockListMonitoredContracts = vi.fn();

vi.mock("@/lib/api", () => ({
  listMonitoredContracts: (...args: unknown[]) =>
    mockListMonitoredContracts(...args),
}));

const contractsPage1: MonitoredContract[] = [
  {
    contract_id: "CA11111111111111111111111111111111111111111111111111111111",
    name: "Stellar Liquidity Pool",
    network: "testnet",
    owner: "G111",
    status: "healthy",
    last_check: null,
    check_interval: 60,
    registered_at: "2026-10-18T00:00:00Z",
    updated_at: "2026-10-18T00:00:00Z",
  },
  {
    contract_id: "CA22222222222222222222222222222222222222222222222222222222",
    name: "Yield Optimizer Vault",
    network: "testnet",
    owner: "G222",
    status: "healthy",
    last_check: null,
    check_interval: 60,
    registered_at: "2026-10-18T00:00:00Z",
    updated_at: "2026-10-18T00:00:00Z",
  },
];

const contractsPage2: MonitoredContract[] = [
  {
    contract_id: "CA33333333333333333333333333333333333333333333333333333333",
    name: "Soroban Governance",
    network: "testnet",
    owner: "G333",
    status: "healthy",
    last_check: null,
    check_interval: 60,
    registered_at: "2026-10-18T00:00:00Z",
    updated_at: "2026-10-18T00:00:00Z",
  },
];

describe("ContractPicker", () => {
  beforeEach(() => {
    mockListMonitoredContracts.mockReset();
  });

  afterEach(() => {
    cleanup();
  });

  it("renders trigger with default placeholder", () => {
    render(<ContractPicker value="" onChange={vi.fn()} />);
    expect(
      screen.getByText("Select or enter a monitored contract (C…)")
    ).toBeInTheDocument();
  });

  it("opens dropdown and loads contracts on click", async () => {
    mockListMonitoredContracts.mockResolvedValueOnce({
      contracts: contractsPage1,
      next_cursor: "cursor_page_2",
    });

    render(<ContractPicker value="" onChange={vi.fn()} />);
    fireEvent.click(screen.getByTestId("contract-picker-trigger"));

    expect(screen.getByTestId("contract-picker-dropdown")).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText("Stellar Liquidity Pool")).toBeInTheDocument();
      expect(screen.getByText("Yield Optimizer Vault")).toBeInTheDocument();
    });
  });

  it("selects a contract when clicked and invokes onChange", async () => {
    mockListMonitoredContracts.mockResolvedValueOnce({
      contracts: contractsPage1,
      next_cursor: "",
    });

    const onChange = vi.fn();
    render(<ContractPicker value="" onChange={onChange} />);

    fireEvent.click(screen.getByTestId("contract-picker-trigger"));
    await waitFor(() =>
      expect(screen.getByText("Stellar Liquidity Pool")).toBeInTheDocument()
    );

    const items = screen.getAllByTestId("contract-picker-item");
    fireEvent.click(items[0]);

    expect(onChange).toHaveBeenCalledWith(contractsPage1[0].contract_id);
    expect(
      screen.queryByTestId("contract-picker-dropdown")
    ).not.toBeInTheDocument();
  });

  it("paginates forward and backward", async () => {
    mockListMonitoredContracts
      .mockResolvedValueOnce({
        contracts: contractsPage1,
        next_cursor: "cursor_page_2",
      })
      .mockResolvedValueOnce({
        contracts: contractsPage2,
        next_cursor: "",
      })
      .mockResolvedValueOnce({
        contracts: contractsPage1,
        next_cursor: "cursor_page_2",
      });

    render(<ContractPicker value="" onChange={vi.fn()} />);
    fireEvent.click(screen.getByTestId("contract-picker-trigger"));

    await waitFor(() =>
      expect(screen.getByText("Stellar Liquidity Pool")).toBeInTheDocument()
    );

    const nextBtn = screen.getByTestId("contract-picker-next");
    expect(nextBtn).toBeEnabled();

    // Click Next
    fireEvent.click(nextBtn);

    await waitFor(() => {
      expect(screen.getByText("Soroban Governance")).toBeInTheDocument();
      expect(screen.getByText("Page 2")).toBeInTheDocument();
    });

    // Previous should now be enabled, Next disabled
    await waitFor(() => {
      expect(screen.getByTestId("contract-picker-prev")).toBeEnabled();
      expect(screen.getByTestId("contract-picker-next")).toBeDisabled();
    });

    const prevBtn = screen.getByTestId("contract-picker-prev");
    // Click Previous
    fireEvent.click(prevBtn);

    await waitFor(() => {
      expect(screen.getByText("Page 1")).toBeInTheDocument();
      expect(screen.getByText("Stellar Liquidity Pool")).toBeInTheDocument();
    });
  });

  it("filters contracts based on search query", async () => {
    mockListMonitoredContracts.mockResolvedValueOnce({
      contracts: contractsPage1,
      next_cursor: "",
    });

    render(<ContractPicker value="" onChange={vi.fn()} />);
    fireEvent.click(screen.getByTestId("contract-picker-trigger"));

    await waitFor(() =>
      expect(screen.getByText("Stellar Liquidity Pool")).toBeInTheDocument()
    );

    const searchInput = screen.getByTestId("contract-picker-search");
    fireEvent.change(searchInput, { target: { value: "Yield" } });

    expect(
      screen.queryByText("Stellar Liquidity Pool")
    ).not.toBeInTheDocument();
    expect(screen.getByText("Yield Optimizer Vault")).toBeInTheDocument();
  });

  it("clears selected contract", () => {
    const onChange = vi.fn();
    render(
      <ContractPicker
        value="CA11111111111111111111111111111111111111111111111111111111"
        onChange={onChange}
      />
    );

    const clearBtn = screen.getByTestId("contract-picker-clear");
    fireEvent.click(clearBtn);

    expect(onChange).toHaveBeenCalledWith("");
  });
});
