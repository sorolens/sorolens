/**
 * Tests for apps/web/components/StorageDiffPanel.tsx
 *
 * We mock @/lib/api so no network is needed, then drive the ledger-range
 * picker and assert the typed two-column diff renders.
 */

import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { StorageDiffPanel } from "./StorageDiffPanel";
import type { DiffStorageEntry, StorageDiffResponse } from "@/lib/types";

const { mockGetDiff } = vi.hoisted(() => ({ mockGetDiff: vi.fn() }));

vi.mock("@/lib/api", () => ({
  ApiError: class ApiError extends Error {},
  getContractStorageDiff: mockGetDiff,
}));

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

const CONTRACT = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA";
const ADDRESS_BEFORE = "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF";
const ADDRESS_AFTER = "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHB";

function entry(
  overrides: Partial<DiffStorageEntry> & { key_xdr: string },
): DiffStorageEntry {
  return {
    contract_id: CONTRACT,
    network: "testnet",
    key_decoded: overrides.key_xdr,
    value_xdr: "AAAA",
    value_decoded: null,
    durability: "persistent",
    live_until_ledger: null,
    last_modified_ledger: 3000,
    status: "live",
    ...overrides,
  };
}

const DIFF: StorageDiffResponse = {
  contract_id: CONTRACT,
  from_ledger: 2000,
  to_ledger: 3000,
  counts: { created: 1, updated: 1, expired: 1 },
  changes: [
    {
      key_xdr: "keyCreated",
      key_decoded: "counter",
      kind: "created",
      durability: "instance",
      before: null,
      after: entry({
        key_xdr: "keyCreated",
        value_decoded: 42,
        durability: "instance",
        last_modified_ledger: 2500,
      }),
      last_modified_ledger: 2500,
    },
    {
      key_xdr: "keyUpdated",
      key_decoded: "admin",
      kind: "updated",
      durability: "persistent",
      changed_fields: ["value"],
      before: entry({ key_xdr: "keyUpdated", value_decoded: ADDRESS_BEFORE }),
      after: entry({ key_xdr: "keyUpdated", value_decoded: ADDRESS_AFTER }),
      last_modified_ledger: 3000,
    },
    {
      key_xdr: "keyExpired",
      key_decoded: "cache",
      kind: "expired",
      durability: "temporary",
      before: entry({
        key_xdr: "keyExpired",
        value_decoded: "stale",
        durability: "temporary",
      }),
      after: null,
      last_modified_ledger: 1200,
    },
  ],
};

function inputValue(label: string): string {
  return (screen.getByLabelText(label) as HTMLInputElement).value;
}

describe("StorageDiffPanel", () => {
  it("prefills a default range and renders a typed diff", async () => {
    mockGetDiff.mockResolvedValue(DIFF);

    render(<StorageDiffPanel contractId={CONTRACT} currentLedger={3000} />);

    await waitFor(() => expect(inputValue("From ledger")).toBe("2000"));
    expect(inputValue("To ledger")).toBe("3000");

    fireEvent.click(screen.getByRole("button", { name: "Compare" }));

    await waitFor(() => expect(screen.queryByText("counter")).not.toBeNull());

    expect(mockGetDiff).toHaveBeenCalledWith(CONTRACT, 2000, 3000);
    expect(screen.queryByText("Created")).not.toBeNull();
    expect(screen.queryByText("Updated")).not.toBeNull();
    expect(screen.queryByText("Expired")).not.toBeNull();
    // The created value is rendered as a typed integer.
    expect(screen.queryByText("42")).not.toBeNull();
    expect(screen.queryByText("integer")).not.toBeNull();
    // Both sides of the updated address entry are labelled as addresses.
    expect(screen.getAllByText("address")).toHaveLength(2);
    // Two columns per change, one per kind.
    expect(screen.getAllByText("Before")).toHaveLength(3);
    expect(screen.getAllByText("After")).toHaveLength(3);
  });

  it("rejects an inverted ledger range before calling the API", async () => {
    mockGetDiff.mockResolvedValue(DIFF);

    render(<StorageDiffPanel contractId={CONTRACT} currentLedger={3000} />);
    await waitFor(() => expect(inputValue("From ledger")).toBe("2000"));

    fireEvent.change(screen.getByLabelText("From ledger"), {
      target: { value: "5000" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Compare" }));

    expect(
      screen.queryByText("'from' must be less than or equal to 'to'"),
    ).not.toBeNull();
    expect(mockGetDiff).not.toHaveBeenCalled();
  });

  it("surfaces an error when the diff request fails", async () => {
    mockGetDiff.mockRejectedValue(new Error("network"));

    render(<StorageDiffPanel contractId={CONTRACT} currentLedger={3000} />);
    await waitFor(() => expect(inputValue("From ledger")).toBe("2000"));

    fireEvent.click(screen.getByRole("button", { name: "Compare" }));

    await waitFor(() =>
      expect(screen.getByRole("alert").textContent ?? "").toContain(
        "Failed to load storage diff",
      ),
    );
  });
});
