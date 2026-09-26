/**
 * Tests for components/compare/CompareView.tsx
 *
 * vitest + @testing-library/react. next/navigation and @/lib/api are mocked so
 * the component can be driven with deterministic data; recharts is stubbed to
 * plain elements because jsdom has no layout engine.
 */

import * as matchers from "@testing-library/jest-dom/matchers";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mockReplace = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: mockReplace, push: vi.fn(), prefetch: vi.fn() }),
}));

const mockListContractsAll = vi.fn();
const mockGetCompare = vi.fn();

vi.mock("@/lib/api", () => ({
  listContractsAll: (...args: unknown[]) => mockListContractsAll(...args),
  getCompare: (...args: unknown[]) => mockGetCompare(...args),
}));

vi.mock("recharts", () => ({
  ResponsiveContainer: ({ children }: { children?: React.ReactNode }) => (
    <div data-testid="recharts-container">{children}</div>
  ),
  AreaChart: ({ children }: { children?: React.ReactNode }) => (
    <div>{children}</div>
  ),
  Area: () => <div data-testid="recharts-area" />,
}));

import { CompareView } from "./CompareView";
import type { CompareContractEntry, ContractSummary } from "@/lib/types";

expect.extend(matchers);

const ID_A = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA";
const ID_B = "CBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB";
const ID_C = "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC";

function contract(id: string, label: string, network: string): ContractSummary {
  return {
    id,
    network,
    label,
    status: "active",
    wasm_hash: null,
    added_at: "2026-09-01T00:00:00Z",
    last_activity_at: null,
  };
}

const CONTRACTS = [
  contract(ID_A, "Alpha", "testnet"),
  contract(ID_B, "Beta", "mainnet"),
  contract(ID_C, "Gamma", "futurenet"),
];

function entry(overrides: Partial<CompareContractEntry>): CompareContractEntry {
  return {
    id: ID_A,
    network: "testnet",
    label: "Alpha",
    status: "active",
    tracked: true,
    has_data: true,
    event_count: 100,
    invocation_count: 50,
    avg_cpu: 1000,
    avg_fee: 200,
    health_score: 90,
    last_synced_ledger: 10,
    event_volume: [{ timestamp: "2026-09-25T00:00:00Z", count: 5 }],
    ...overrides,
  };
}

describe("CompareView", () => {
  beforeEach(() => {
    mockReplace.mockReset();
    mockListContractsAll.mockReset();
    mockGetCompare.mockReset();
    mockListContractsAll.mockResolvedValue({ contracts: CONTRACTS });
    mockGetCompare.mockResolvedValue({
      window: "7d",
      contracts: [
        entry({}),
        entry({
          id: ID_B,
          network: "mainnet",
          label: "Beta",
          event_count: 10,
          avg_cpu: 5000,
          avg_fee: 400,
          health_score: 40,
        }),
      ],
    });
  });

  afterEach(cleanup);

  it("fetches the comparison in one round-trip and renders both columns", async () => {
    render(<CompareView initialIds={[ID_A, ID_B]} initialWindow="7d" />);

    await waitFor(() =>
      expect(screen.getByText("90% worse")).toBeInTheDocument()
    );

    expect(mockGetCompare).toHaveBeenCalledWith([ID_A, ID_B], "7d");
    // Both columns rendered.
    expect(screen.getAllByText("Alpha").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Beta").length).toBeGreaterThan(0);
    // Both slots are filled in the picker.
    expect(screen.getAllByTestId("compare-slot-filled")).toHaveLength(2);
  });

  it("mirrors the selection into a shareable URL", async () => {
    render(<CompareView initialIds={[ID_A, ID_B]} initialWindow="7d" />);

    await waitFor(() =>
      expect(mockReplace).toHaveBeenCalledWith(`/compare?ids=${ID_A},${ID_B}`, {
        scroll: false,
      })
    );
  });

  it("renders a contract with no data yet without failing the comparison", async () => {
    mockListContractsAll.mockResolvedValue({ contracts: CONTRACTS });
    mockGetCompare.mockResolvedValue({
      window: "7d",
      contracts: [
        entry({}),
        entry({
          id: ID_C,
          network: "futurenet",
          label: "Gamma",
          tracked: true,
          has_data: false,
          event_count: 0,
          invocation_count: 0,
          avg_cpu: 0,
          avg_fee: 0,
          health_score: null,
          event_volume: [],
        }),
      ],
    });

    render(<CompareView initialIds={[ID_A, ID_C]} initialWindow="7d" />);

    await waitFor(() =>
      expect(
        screen.getByText("No data yet for this contract.")
      ).toBeInTheDocument()
    );
    expect(screen.getByTestId("sparkline-empty")).toBeInTheDocument();
  });
});
