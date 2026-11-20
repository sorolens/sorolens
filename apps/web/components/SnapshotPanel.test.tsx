/**
 * Tests for apps/web/components/SnapshotPanel.tsx (issue #765).
 *
 * The panel downloads one snapshot per asked ledger, so the tests mock
 * `getContractSnapshot`: they check the request arguments, the rendered
 * storage/event rows, the loading state, and that an API failure is shown with
 * the server's message rather than an empty table.
 */

import "@testing-library/jest-dom/vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ContractSnapshot, SnapshotStorageEntry } from "@/lib/types";
import { ApiError } from "@/lib/api";
import { SnapshotPanel } from "./SnapshotPanel";

const mockGetContractSnapshot = vi.fn();

vi.mock("@/lib/api", () => ({
  // Mirrors the real class so `err instanceof ApiError` still holds.
  ApiError: class ApiError extends Error {
    constructor(
      public status: number,
      message: string,
      public code?: string
    ) {
      super(message);
      this.name = "ApiError";
    }
  },
  getContractSnapshot: (...args: unknown[]) => mockGetContractSnapshot(...args),
}));

const CONTRACT_ID = `C${"A".repeat(55)}`;

function storageEntry(
  overrides: Partial<SnapshotStorageEntry> = {}
): SnapshotStorageEntry {
  return {
    key_xdr: "AAAAFAKEKEY",
    key_decoded: "balance",
    value_xdr: "AAAABVALUE",
    value_decoded: 1250,
    durability: "persistent",
    live_until_ledger: 900,
    ledgers_until_expiry: 200,
    status: "live",
    last_modified_ledger: 640,
    network: "testnet",
    ...overrides,
  };
}

function snapshot(overrides: Partial<ContractSnapshot> = {}): ContractSnapshot {
  return {
    contract_id: CONTRACT_ID,
    network: "testnet",
    ledger: 1000,
    first_tracked_ledger: 512,
    storage: [storageEntry()],
    last_event: {
      id: "event-1",
      ledger: 990,
      tx_hash: "abc123",
      type: "contract",
      ledger_closed_at: "2026-09-01T00:00:00Z",
      value_decoded: null,
      value_xdr: "AAAA",
    },
    ...overrides,
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function renderPanel() {
  render(<SnapshotPanel contractId={CONTRACT_ID} currentLedger={12345} />);
  return {
    input: screen.getByLabelText<HTMLInputElement>("Replay state at ledger"),
    button: screen.getByRole("button", { name: "View snapshot" }),
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  mockGetContractSnapshot.mockResolvedValue(snapshot());
});

afterEach(cleanup);

describe("SnapshotPanel", () => {
  it("starts with an empty prompt and a disabled button", () => {
    const { input, button } = renderPanel();

    expect(input.value).toBe("");
    expect(input).toHaveAttribute("placeholder", "12345");
    expect(button).toBeDisabled();
    expect(screen.getByText(/Enter a ledger to see/)).toBeVisible();
  });

  it("prefills the placeholder from the latest indexed ledger", async () => {
    render(<SnapshotPanel contractId={CONTRACT_ID} currentLedger={0} />);

    expect(screen.getByLabelText("Replay state at ledger")).toHaveAttribute(
      "placeholder",
      "e.g. 12345"
    );
  });

  it("downloads the snapshot for the requested ledger", async () => {
    const { input, button } = renderPanel();

    fireEvent.change(input, { target: { value: "1000" } });
    fireEvent.click(button);

    expect(await screen.findByText("balance")).toBeVisible();
    expect(mockGetContractSnapshot).toHaveBeenCalledWith(CONTRACT_ID, 1000);
    // Numbers are formatted for the reader, not printed raw.
    expect(screen.getByText(/First tracked ledger:/)).toHaveTextContent(
      `First tracked ledger: ${
        (512).toLocaleString()
      }`
    );
  });

  it("renders the last known event and the TTL columns", async () => {
    const { input, button } = renderPanel();

    fireEvent.change(input, { target: { value: "1000" } });
    fireEvent.click(button);
    await screen.findByText("balance");

    expect(
      screen.getByText(/Last known event at or before ledger/)
    ).toBeVisible();
    expect(screen.getByText(/event-1/)).toBeVisible();
    expect(screen.getByText(/abc123/)).toBeVisible();
    expect(
      screen.getByRole("columnheader", { name: "Durability" })
    ).toBeVisible();
    expect(screen.getByText("persistent")).toBeVisible();
    expect(screen.getByText("640")).toBeVisible();
    expect(screen.getByText("AAAABVALUE")).toBeVisible();
  });

  it("explains an empty storage set at that ledger", async () => {
    mockGetContractSnapshot.mockResolvedValue(
      snapshot({ storage: [], last_event: null })
    );
    const { input, button } = renderPanel();

    fireEvent.change(input, { target: { value: "1000" } });
    fireEvent.click(button);

    expect(
      await screen.findByText(/No storage entries were live at ledger/)
    ).toBeVisible();
    expect(
      screen.getByText(/No event was recorded at or before ledger/)
    ).toBeVisible();
    expect(screen.queryByRole("table")).toBeNull();
  });

  it("rejects a non-positive ledger without calling the API", async () => {
    const { input, button } = renderPanel();

    fireEvent.change(input, { target: { value: "0" } });
    fireEvent.click(button);

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Enter a positive ledger number"
    );
    expect(mockGetContractSnapshot).not.toHaveBeenCalled();
  });

  it("rejects a fractional ledger without calling the API", () => {
    const { input, button } = renderPanel();

    fireEvent.change(input, { target: { value: "1.5" } });
    fireEvent.click(button);

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Enter a positive ledger number"
    );
    expect(mockGetContractSnapshot).not.toHaveBeenCalled();
  });

  it("surfaces the API's own error message", async () => {
    mockGetContractSnapshot.mockRejectedValue(
      new ApiError(404, "No snapshot exists for that ledger.", "not_found")
    );
    const { input, button } = renderPanel();

    fireEvent.change(input, { target: { value: "42" } });
    fireEvent.click(button);

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "No snapshot exists for that ledger."
    );
    expect(screen.queryByText("balance")).toBeNull();
  });

  it("falls back to a generic message for a non-ApiError failure", async () => {
    mockGetContractSnapshot.mockRejectedValue(new TypeError("network down"));
    const { input, button } = renderPanel();

    fireEvent.change(input, { target: { value: "42" } });
    fireEvent.click(button);

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Failed to load snapshot for that ledger"
    );
  });

  it("clears a previous error when the ledger is edited", async () => {
    mockGetContractSnapshot.mockRejectedValue(
      new ApiError(500, "Indexer unavailable.")
    );
    const { input, button } = renderPanel();

    fireEvent.change(input, { target: { value: "42" } });
    fireEvent.click(button);
    await screen.findByRole("alert");

    fireEvent.change(input, { target: { value: "43" } });

    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("shows a loading state while the request is in flight", async () => {
    const pending = deferred<ContractSnapshot>();
    mockGetContractSnapshot.mockReturnValue(pending.promise);
    const { input } = renderPanel();

    fireEvent.change(input, { target: { value: "1000" } });
    fireEvent.click(screen.getByRole("button", { name: "View snapshot" }));

    const busy = screen.getByRole("button", { name: "Loading…" });
    expect(busy).toBeDisabled();
    expect(busy).toHaveAttribute("aria-busy", "true");

    pending.resolve(snapshot());
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "View snapshot" })
      ).toBeEnabled()
    );
    expect(screen.getByText("balance")).toBeVisible();
  });
});
