/**
 * Tests for apps/web/app/(app)/watchdog/page.tsx monitored-contracts paging.
 *
 * We use vitest + @testing-library/react + jsdom.
 * next/link is mocked to a plain <a> so we don't need the Next.js runtime.
 * @/lib/api is mocked so we control the pages returned.
 */

import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NetworkProvider, useNetwork } from "@/lib/network";
import type { MonitoredContract } from "@/lib/types";

// ── Mock next/link ──────────────────────────────────────────────────────────
vi.mock("next/link", () => ({
  default: ({
    children,
    href,
  }: {
    children: React.ReactNode;
    href: string;
  }) => <a href={href}>{children}</a>,
}));

// ── Mock @/lib/api ───────────────────────────────────────────────────────────
const mockListMonitoredContracts = vi.fn();

vi.mock("@/lib/api", () => ({
  getWatchdogStats: () =>
    Promise.resolve({
      total_monitored: 0,
      healthy: 0,
      degraded: 0,
      unresponsive: 0,
      total_alerts: 0,
      critical_alerts: 0,
    }),
  listAlerts: () => Promise.resolve({ alerts: [] }),
  listMonitoredContracts: (...args: unknown[]) =>
    mockListMonitoredContracts(...args),
}));

// ── Mock @/components/Skeleton ───────────────────────────────────────────────
vi.mock("@/components/Skeleton", () => ({
  TableSkeleton: () => <div data-testid="table-skeleton">skeleton</div>,
}));

// ── Fixtures ─────────────────────────────────────────────────────────────────

function monitored(name: string): MonitoredContract {
  return {
    contract_id: `C${name.toUpperCase().padEnd(55, "A")}`,
    network: "testnet",
    name,
    owner: "GOWNER",
    status: "Healthy",
    last_check: null,
    check_interval: 60,
    registered_at: "2025-01-01T00:00:00Z",
    updated_at: "2025-01-01T00:00:00Z",
  };
}

// Three pages keyed by the cursor the page must send to get them.
const PAGES: Record<
  string,
  { contracts: MonitoredContract[]; next_cursor: string }
> = {
  first: {
    contracts: [monitored("alpha"), monitored("bravo")],
    next_cursor: "cursor-2",
  },
  "cursor-2": {
    contracts: [monitored("charlie"), monitored("delta")],
    next_cursor: "cursor-3",
  },
  "cursor-3": { contracts: [monitored("echo")], next_cursor: "" },
};

// ── Helpers ──────────────────────────────────────────────────────────────────

function nextButton() {
  return screen.getByRole<HTMLButtonElement>("button", { name: "Next page" });
}

function prevButton() {
  return screen.getByRole<HTMLButtonElement>("button", {
    name: "Previous page",
  });
}

function lastCallCursor(): string | undefined {
  const calls = mockListMonitoredContracts.mock.calls;
  return (calls[calls.length - 1][0] as { cursor?: string }).cursor;
}

/** Switches the header network, as the NetworkSelector would. */
function NetworkSwitcher() {
  const { setNetwork } = useNetwork();
  return (
    <button type="button" onClick={() => setNetwork("mainnet")}>
      switch network
    </button>
  );
}

async function renderPage() {
  const { default: WatchdogPage } = await import("@/app/(app)/watchdog/page");
  return render(
    <NetworkProvider>
      <NetworkSwitcher />
      <WatchdogPage />
    </NetworkProvider>
  );
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe("WatchdogPage monitored contracts pagination", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListMonitoredContracts.mockImplementation(
      (params?: { cursor?: string }) =>
        Promise.resolve(PAGES[params?.cursor ?? "first"])
    );
  });

  afterEach(() => {
    cleanup();
  });

  it("renders the first page and requests it without a cursor", async () => {
    await renderPage();

    await screen.findByText("alpha");
    expect(screen.getByText("bravo")).toBeDefined();
    expect(screen.getByText("Page 1")).toBeDefined();
    expect(mockListMonitoredContracts).toHaveBeenCalledTimes(1);
    expect(mockListMonitoredContracts).toHaveBeenCalledWith({
      cursor: undefined,
      limit: 20,
      network: undefined,
    });
    expect(prevButton().disabled).toBe(true);
    expect(nextButton().disabled).toBe(false);
  });

  it("requests the next page with the returned next_cursor", async () => {
    await renderPage();
    await screen.findByText("alpha");

    fireEvent.click(nextButton());

    await screen.findByText("charlie");
    expect(lastCallCursor()).toBe("cursor-2");
    expect(screen.queryByText("alpha")).toBeNull();
    expect(screen.getByText("Page 2")).toBeDefined();
    expect(prevButton().disabled).toBe(false);
  });

  it("disables next on the last page, when next_cursor is empty", async () => {
    await renderPage();
    await screen.findByText("alpha");

    fireEvent.click(nextButton());
    await screen.findByText("charlie");
    fireEvent.click(nextButton());
    await screen.findByText("echo");

    expect(lastCallCursor()).toBe("cursor-3");
    expect(screen.getByText("Page 3")).toBeDefined();
    expect(nextButton().disabled).toBe(true);
  });

  it("goes back to the previous page with that page's cursor", async () => {
    await renderPage();
    await screen.findByText("alpha");

    fireEvent.click(nextButton());
    await screen.findByText("charlie");
    fireEvent.click(nextButton());
    await screen.findByText("echo");

    fireEvent.click(prevButton());
    await screen.findByText("charlie");
    expect(lastCallCursor()).toBe("cursor-2");
    expect(screen.getByText("Page 2")).toBeDefined();

    fireEvent.click(prevButton());
    await screen.findByText("alpha");
    expect(lastCallCursor()).toBeUndefined();
    expect(prevButton().disabled).toBe(true);
  });

  it("returns to the first page when the network changes", async () => {
    await renderPage();
    await screen.findByText("alpha");
    fireEvent.click(nextButton());
    await screen.findByText("charlie");

    fireEvent.click(screen.getByRole("button", { name: "switch network" }));

    await screen.findByText("alpha");
    expect(screen.getByText("Page 1")).toBeDefined();
    expect(mockListMonitoredContracts).toHaveBeenLastCalledWith({
      cursor: undefined,
      limit: 20,
      network: "mainnet",
    });
  });

  it("shows a skeleton while a page is loading", async () => {
    mockListMonitoredContracts.mockReturnValue(new Promise(() => {}));
    await renderPage();
    // Alerts settle; only the monitored-contracts skeleton remains.
    await screen.findByText("No alerts yet");
    expect(screen.getAllByTestId("table-skeleton")).toHaveLength(1);
    expect(screen.queryByRole("button", { name: "Next page" })).toBeNull();
  });

  it("shows the empty state without pagination when there are no contracts", async () => {
    mockListMonitoredContracts.mockResolvedValue({
      contracts: [],
      next_cursor: "",
    });
    await renderPage();

    await screen.findByText("No monitored contracts yet");
    expect(screen.queryByRole("button", { name: "Next page" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Previous page" })).toBeNull();
  });

  it("shows the empty state when the API is unavailable", async () => {
    mockListMonitoredContracts.mockRejectedValue(new Error("network down"));
    await renderPage();

    await screen.findByText("No monitored contracts yet");
    expect(screen.queryByRole("button", { name: "Next page" })).toBeNull();
  });
});
