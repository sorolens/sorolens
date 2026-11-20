/**
 * Tests for apps/web/components/StoragePanel.tsx (issue #765).
 *
 * The panel sorts by TTL urgency and filters by durability, so these tests pin
 * both: a danger entry must come first regardless of input order, and the
 * filter must not hide rows the user filtered for. Pure rendering, no network.
 */

import "@testing-library/jest-dom/vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { StoragePanel } from "./StoragePanel";
import type { StorageEntry } from "@/lib/types";

afterEach(cleanup);

function entry(overrides: Partial<StorageEntry> = {}): StorageEntry {
  return {
    key_xdr: "AAAAFAKEKEY000000000000000000000000000000000000000000",
    key_decoded: "balance",
    value_xdr: "AAAAVALUE",
    value_decoded: null,
    durability: "persistent",
    live_until_ledger: 1200,
    ledgers_until_expiry: 1000,
    status: "live",
    last_modified_ledger: 900,
    ...overrides,
  };
}

function renderPanel(
  entries: StorageEntry[],
  overrides: Partial<Parameters<typeof StoragePanel>[0]> = {}
) {
  render(
    <StoragePanel entries={entries} currentLedger={1000} {...overrides} />
  );
}

/** First column of every body row, in render order. */
function renderedKeys(): string[] {
  return screen
    .getAllByRole("row")
    .slice(1)
    .map((row) => row.querySelector("td")?.textContent ?? "");
}

describe("StoragePanel", () => {
  it("renders an empty state when there are no entries", () => {
    renderPanel([]);

    expect(screen.getByText("No storage entries found")).toBeVisible();
    expect(screen.queryByRole("table")).toBeNull();
  });

  it("keeps the table while loading, even with no entries yet", () => {
    renderPanel([], { loading: true });

    expect(screen.queryByText("No storage entries found")).toBeNull();
    expect(screen.getByRole("table")).toBeVisible();
    expect(screen.getByText(/Current ledger:/)).toBeVisible();
  });

  it("renders the decoded key, durability, TTL and age", () => {
    renderPanel([
      entry({
        ledgers_until_expiry: 34560,
        last_modified_ledger: 400,
        durability: "temporary",
      }),
    ]);

    const row = screen.getAllByRole("row")[1];
    expect(within(row).getByText("balance")).toBeVisible();
    // “temporary” is also a filter button, so scope the assertion to the row.
    expect(within(row).getByText("temporary")).toBeVisible();
    expect(within(row).getByText((34560).toLocaleString())).toBeVisible();
    expect(within(row).getByText((1000 - 400).toLocaleString())).toBeVisible();
    expect(screen.getByText(/Current ledger:/)).toHaveTextContent(
      (1000).toLocaleString()
    );
  });

  it("falls back to a truncated XDR key and dashes for unknown values", () => {
    renderPanel([
      entry({
        key_decoded: null,
        ledgers_until_expiry: null,
        last_modified_ledger: null,
      }),
    ]);

    const key = renderedKeys()[0];
    expect(key).toContain("...");
    expect(key).not.toBe(
      "AAAAFAKEKEY000000000000000000000000000000000000000000"
    );
    // TTL and age both render a dash when the indexer did not report them.
    expect(screen.getAllByText("-")).toHaveLength(2);
  });

  it("sorts danger entries before warning and safe ones", () => {
    renderPanel([
      entry({ key_decoded: "safe", ledgers_until_expiry: 500_000 }),
      entry({ key_decoded: "warning", ledgers_until_expiry: 50_000 }),
      entry({ key_decoded: "danger", ledgers_until_expiry: 1_000 }),
    ]);

    expect(renderedKeys()).toEqual(["danger", "warning", "safe"]);
  });

  it("filters by durability without losing the rest of the table", () => {
    renderPanel([
      entry({ key_decoded: "persist", durability: "persistent" }),
      entry({ key_decoded: "temp", durability: "temporary" }),
      entry({ key_decoded: "inst", durability: "instance" }),
    ]);

    fireEvent.click(screen.getByRole("button", { name: "temporary" }));

    expect(renderedKeys()).toEqual(["temp"]);
    expect(screen.queryByText("persist")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "all" }));

    expect(renderedKeys()).toHaveLength(3);
  });

  it("shows a filter with no matches as an empty table, not the empty state", () => {
    renderPanel([entry({ durability: "persistent" })]);

    fireEvent.click(screen.getByRole("button", { name: "instance" }));

    expect(renderedKeys()).toEqual([]);
    expect(screen.getByRole("table")).toBeVisible();
    expect(screen.queryByText("No storage entries found")).toBeNull();
  });

  it("loads more pages through the callback", () => {
    const onLoadMore = vi.fn();
    renderPanel([entry()], { hasMore: true, onLoadMore });

    fireEvent.click(screen.getByRole("button", { name: "Load More" }));

    expect(onLoadMore).toHaveBeenCalledTimes(1);
  });

  it("disables Load More while a page is loading", () => {
    renderPanel([entry()], {
      hasMore: true,
      onLoadMore: vi.fn(),
      loading: true,
    });

    const button = screen.getByRole("button", { name: "Loading..." });
    expect(button).toBeDisabled();
    expect(button).toHaveAttribute("aria-busy", "true");
  });

  it("hides Load More when there are no further pages", () => {
    renderPanel([entry()]);

    expect(screen.queryByRole("button", { name: /Load More/ })).toBeNull();
  });

  it("documents the TTL thresholds in the legend", () => {
    renderPanel([entry()]);

    expect(screen.getByText(/Safe \(>7d\)/)).toBeVisible();
    expect(screen.getByText(/Warning \(2-7d\)/)).toBeVisible();
    expect(screen.getByText(/Danger \(<2d\)/)).toBeVisible();
  });
});
