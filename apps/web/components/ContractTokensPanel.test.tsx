import * as matchers from "@testing-library/jest-dom/matchers";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ContractTokensPanel } from "./ContractTokensPanel";

expect.extend(matchers);

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
  }: {
    children: React.ReactNode;
    href: string;
  }) => <a href={href}>{children}</a>,
}));
vi.mock("@sorolens/ui", () => ({
  MonoId: ({ value }: { value: string }) => <span>{value}</span>,
}));
vi.mock("recharts", () => {
  const Stub = ({ children }: { children?: React.ReactNode }) => (
    <div>{children}</div>
  );
  return {
    Bar: Stub,
    BarChart: Stub,
    CartesianGrid: Stub,
    ResponsiveContainer: Stub,
    Tooltip: Stub,
    XAxis: Stub,
    YAxis: Stub,
  };
});

const mockTransfers = vi.fn();
const mockHolders = vi.fn();
vi.mock("@/lib/tokens", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/tokens")>()),
  getTokenTransfers: (...a: unknown[]) => mockTransfers(...a),
  getTokenHolders: (...a: unknown[]) => mockHolders(...a),
}));

const transfer = (id: string, overrides = {}) => ({
  id,
  kind: "transfer",
  from: "GAAA",
  to: "GBBB",
  amount: "25000000",
  tx_hash: `hash${id}000000000`,
  ledger: 120400,
  ledger_closed_at: "2026-07-03T08:00:00Z",
  ...overrides,
});

const holdersPage = {
  holders: [
    { address: "GHOLDERONE", balance: "75000000" },
    { address: "GHOLDERTWO", balance: "25000000" },
  ],
  next_cursor: "",
  decimals: 7,
  total_supply: "100000000",
};

beforeEach(() => {
  mockTransfers.mockReset();
  mockHolders.mockReset();
  mockTransfers.mockResolvedValue({
    transfers: [transfer("1")],
    next_cursor: "",
    decimals: 7,
  });
  mockHolders.mockResolvedValue(holdersPage);
});
afterEach(cleanup);

const transfersSection = () =>
  screen.getByRole("region", { name: "Transfers" });
const holdersSection = () => screen.getByRole("region", { name: "Holders" });

describe("ContractTokensPanel", () => {
  it("formats amounts and links transfers to their transaction", async () => {
    render(<ContractTokensPanel contractId="C1" />);

    expect(
      await within(transfersSection()).findByText("2.5")
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /hash1/ })).toHaveAttribute(
      "href",
      "/search?q=hash1000000000"
    );
  });

  it("shows each holder's share of supply and the distribution", async () => {
    render(<ContractTokensPanel contractId="C1" />);

    expect(await screen.findByText("75.00%")).toBeInTheDocument();
    expect(screen.getByText("25.00%")).toBeInTheDocument();
    expect(screen.getByTestId("holder-distribution")).toBeInTheDocument();
  });

  it("filters transfers by address and amount", async () => {
    mockTransfers.mockResolvedValue({
      transfers: [
        transfer("1"),
        transfer("2", { to: "GCCC", amount: "90000000" }),
      ],
      next_cursor: "",
    });
    render(<ContractTokensPanel contractId="C1" />);
    await screen.findByText("9");
    const section = within(transfersSection());

    fireEvent.change(section.getByLabelText("Address"), {
      target: { value: "GCCC" },
    });
    expect(section.queryByText("2.5")).not.toBeInTheDocument();
    expect(section.getByText("9")).toBeInTheDocument();

    fireEvent.change(section.getByLabelText("Address"), {
      target: { value: "" },
    });
    fireEvent.change(section.getByLabelText("Minimum amount"), {
      target: { value: "5" },
    });
    expect(section.queryByText("2.5")).not.toBeInTheDocument();

    fireEvent.change(section.getByLabelText("Minimum amount"), {
      target: { value: "abc" },
    });
    expect(section.getByRole("alert")).toHaveTextContent(
      "Enter a valid amount."
    );
  });

  it("paginates with the returned cursor", async () => {
    mockTransfers
      .mockResolvedValueOnce({ transfers: [transfer("1")], next_cursor: "c2" })
      .mockResolvedValueOnce({
        transfers: [transfer("2", { amount: "90000000" })],
        next_cursor: "",
      });
    render(<ContractTokensPanel contractId="C1" />);
    await within(transfersSection()).findByText("2.5");

    fireEvent.click(
      within(transfersSection()).getByRole("button", { name: "Load more" })
    );

    expect(
      await within(transfersSection()).findByText("9")
    ).toBeInTheDocument();
    expect(mockTransfers).toHaveBeenLastCalledWith("C1", {
      cursor: "c2",
      limit: 25,
    });
    expect(
      within(transfersSection()).queryByRole("button", { name: "Load more" })
    ).not.toBeInTheDocument();
  });

  it("exports the filtered rows as CSV", async () => {
    const createObjectURL = vi.fn((blob: Blob) => {
      void blob;
      return "blob:csv";
    });
    vi.stubGlobal("URL", Object.assign(URL, { createObjectURL }));
    URL.revokeObjectURL = vi.fn();
    const click = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => {});

    render(<ContractTokensPanel contractId="C1" />);
    await screen.findByText("75.00%");
    fireEvent.click(
      within(holdersSection()).getByRole("button", { name: "Export CSV" })
    );

    expect(click).toHaveBeenCalledTimes(1);
    expect(createObjectURL.mock.calls[0][0].type).toBe("text/csv");
    click.mockRestore();
    vi.unstubAllGlobals();
  });

  it("shows empty states", async () => {
    mockTransfers.mockResolvedValue({ transfers: [], next_cursor: "" });
    mockHolders.mockResolvedValue({ holders: [], next_cursor: "" });
    render(<ContractTokensPanel contractId="C1" />);

    expect(
      await screen.findByText("No transfers indexed yet.")
    ).toBeInTheDocument();
    expect(
      await screen.findByText("No holders indexed yet.")
    ).toBeInTheDocument();
  });

  it("shows an error with a working retry", async () => {
    mockHolders.mockRejectedValueOnce(new Error("boom"));
    render(<ContractTokensPanel contractId="C1" />);

    expect(
      await screen.findByText("Couldn't load holders.")
    ).toBeInTheDocument();
    fireEvent.click(
      within(holdersSection()).getByRole("button", { name: /try again/i })
    );

    await waitFor(() => expect(screen.getByText("75.00%")).toBeInTheDocument());
  });
});
