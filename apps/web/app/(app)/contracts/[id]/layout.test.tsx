import { describe, expect, it, vi } from "vitest";
import { getContractForMetadata } from "@/lib/serverData";

vi.mock("@/lib/serverData", () => ({
  getContractForMetadata: vi.fn(),
}));

import { generateMetadata } from "./layout";

describe("contract metadata", () => {
  it("uses the contract label and network in the page title", async () => {
    vi.mocked(getContractForMetadata).mockResolvedValue({
      id: "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
      network: "mainnet",
      label: "Token Vault",
      status: "active",
      wasm_hash: null,
      created_at_ledger: 0,
      backfill_complete_at: null,
      tags: [],
      added_at: "2026-01-01T00:00:00Z",
    });

    const metadata = await generateMetadata({
      params: Promise.resolve({
        id: "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
      }),
    });

    expect(metadata.title).toBe("Token Vault (mainnet)");
    expect(metadata.description).toContain("Token Vault on mainnet");
  });

  it("keeps a useful short-ID title when the API has no contract record", async () => {
    vi.mocked(getContractForMetadata).mockResolvedValue(null);

    const metadata = await generateMetadata({
      params: Promise.resolve({
        id: "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
      }),
    });

    expect(metadata.title).toContain("CAAAAA");
  });
});
