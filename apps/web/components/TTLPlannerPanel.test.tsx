/**
 * Tests for apps/web/components/TTLPlannerPanel.tsx (issue #830)
 */

import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { TTLPlannerPanel } from "./TTLPlannerPanel";
import type { StorageEntry } from "@/lib/types";

afterEach(cleanup);

function makeEntry(
  keyXdr: string,
  ledgersUntilExpiry: number | null
): StorageEntry {
  return {
    key_xdr: keyXdr,
    key_decoded: null,
    value_xdr: null,
    value_decoded: null,
    durability: "persistent",
    live_until_ledger: 1000,
    ledgers_until_expiry: ledgersUntilExpiry,
    status: "live",
    last_modified_ledger: 500,
  };
}

describe("TTLPlannerPanel", () => {
  it("renders when entries are expiring within the threshold", () => {
    const entries = [makeEntry("AAAA", 1000)];
    render(
      <TTLPlannerPanel
        entries={entries}
        currentLedger={1_000_000}
        network="testnet"
      />
    );
    expect(screen.getByText(/TTL Extension Planner/i)).toBeInTheDocument();
  });

  it("renders nothing when no entries are near expiry", () => {
    const entries = [makeEntry("AAAA", 999_999)];
    const { container } = render(
      <TTLPlannerPanel
        entries={entries}
        currentLedger={1_000_000}
        network="testnet"
      />
    );
    expect(container.firstChild).toBeNull();
  });

  it("renders nothing when entries array is empty", () => {
    const { container } = render(
      <TTLPlannerPanel
        entries={[]}
        currentLedger={1_000_000}
        network="testnet"
      />
    );
    expect(container.firstChild).toBeNull();
  });

  it("lists the expiring entry", () => {
    const entries = [makeEntry("EXPIRING_KEY_XDR", 500)];
    render(
      <TTLPlannerPanel
        entries={entries}
        currentLedger={1_000_000}
        network="testnet"
      />
    );
    expect(screen.getByText(/500 ledgers/)).toBeInTheDocument();
  });
});
