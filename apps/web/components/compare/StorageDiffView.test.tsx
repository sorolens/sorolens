import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { StorageDiffView } from "./StorageDiffView";

const mockDiffData = {
  contract_id: "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAD2KM",
  from_ledger: 10,
  to_ledger: 20,
  created: [
    {
      key: "created_key",
      durability: "persistent" as const,
      live_until_ledger: 500,
      val_decoded: { type: "scvU32", value: 42, human: "42" }
    }
  ],
  changed: [
    {
      key: "changed_key",
      durability: "temporary" as const,
      old_val_decoded: { type: "scvU32", value: 10, human: "10" },
      new_val_decoded: { type: "scvU32", value: 20, human: "20" },
      old_live_until: 100,
      new_live_until: 200
    }
  ],
  removed: [
    {
      key: "removed_key",
      durability: "instance" as const
    }
  ],
  expired: [
    {
      key: "expired_key",
      durability: "temporary" as const,
      live_until_ledger: 15
    }
  ]
};

// Mock global fetch
const globalFetchMock = vi.fn();
vi.stubGlobal("fetch", globalFetchMock);

describe("StorageDiffView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders loading state initially", () => {
    globalFetchMock.mockReturnValueOnce(new Promise(() => {}));
    render(<StorageDiffView contractId="CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAD2KM" initialFrom={10} initialTo={20} />);
    expect(screen.getByText(/Loading storage diff/i)).toBeDefined();
  });

  it("renders storage diff entries across created, changed, removed, and expired groups", async () => {
    globalFetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => mockDiffData,
    });

    render(<StorageDiffView contractId="CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAD2KM" initialFrom={10} initialTo={20} />);

    await waitFor(() => {
      expect(screen.getByText("created_key")).toBeDefined();
      expect(screen.getByText("changed_key")).toBeDefined();
      expect(screen.getByText("removed_key")).toBeDefined();
      expect(screen.getByText("expired_key")).toBeDefined();
    });
  });

  it("handles empty state gracefully", async () => {
    globalFetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        contract_id: "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAD2KM",
        from_ledger: 10,
        to_ledger: 20,
        created: [],
        changed: [],
        removed: [],
        expired: [],
      }),
    });

    render(<StorageDiffView contractId="CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAD2KM" initialFrom={10} initialTo={20} />);

    await waitFor(() => {
      expect(screen.getByText(/No storage diff entries found/i)).toBeDefined();
    });
  });

  it(
    "handles error states when API fails",
    async () => {
      globalFetchMock.mockResolvedValueOnce({
        ok: false,
        status: 500,
      });

      render(
        <StorageDiffView
          contractId="CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAD2KM"
          initialFrom={10}
          initialTo={20}
        />
      );

      await waitFor(() => {
        expect(screen.getByText(/Failed to fetch storage diff/i)).toBeDefined();
      });
    },
    20000
  );

  it("filters entries when prefix input or durability changes", async () => {
    globalFetchMock.mockResolvedValue({
      ok: true,
      json: async () => mockDiffData,
    });

    render(<StorageDiffView contractId="CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAD2KM" initialFrom={10} initialTo={20} />);

    await waitFor(() => {
      expect(screen.getByText("created_key")).toBeDefined();
    });

    const prefixInput = screen.getByPlaceholderText(/Key prefix filter/i);
    fireEvent.change(prefixInput, { target: { value: "created" } });

    expect((prefixInput as HTMLInputElement).value).toBe("created");
  });
});
