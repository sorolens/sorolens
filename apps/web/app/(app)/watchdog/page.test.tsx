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
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NetworkProvider, useNetwork } from "@/lib/network";
import type { AlertGroup, ContractAlert, MonitoredContract } from "@/lib/types";

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
const mockListAlertGroups = vi.fn();
const mockListAlerts = vi.fn();

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
  listAlertGroups: (...args: unknown[]) => mockListAlertGroups(...args),
  listAlerts: (...args: unknown[]) => mockListAlerts(...args),
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

/**
 * Waits for a contract name row inside the monitored-contracts table.
 * Contract names also appear as options in the alerts contract filter, so the
 * lookup is scoped to the table. The table unmounts into a skeleton between
 * pages, so the scope is re-resolved on every call.
 */
async function findContractRow(name: string) {
  return within(await screen.findByRole("table")).findByText(name);
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
    // The alerts feed settles empty by default so tests can focus on the
    // monitored-contracts table.
    mockListAlertGroups.mockResolvedValue({ groups: [], next_cursor: "" });
    mockListAlerts.mockResolvedValue({ alerts: [], next_cursor: "" });
  });

  afterEach(() => {
    cleanup();
  });

  it("renders the first page and requests it without a cursor", async () => {
    await renderPage();

    await findContractRow("alpha");
    expect(within(screen.getByRole("table")).getByText("bravo")).toBeDefined();
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
    await findContractRow("alpha");

    fireEvent.click(nextButton());

    await findContractRow("charlie");
    expect(lastCallCursor()).toBe("cursor-2");
    expect(within(screen.getByRole("table")).queryByText("alpha")).toBeNull();
    expect(screen.getByText("Page 2")).toBeDefined();
    expect(prevButton().disabled).toBe(false);
  });

  it("disables next on the last page, when next_cursor is empty", async () => {
    await renderPage();
    await findContractRow("alpha");

    fireEvent.click(nextButton());
    await findContractRow("charlie");
    fireEvent.click(nextButton());
    await findContractRow("echo");

    expect(lastCallCursor()).toBe("cursor-3");
    expect(screen.getByText("Page 3")).toBeDefined();
    expect(nextButton().disabled).toBe(true);
  });

  it("goes back to the previous page with that page's cursor", async () => {
    await renderPage();
    await findContractRow("alpha");

    fireEvent.click(nextButton());
    await findContractRow("charlie");
    fireEvent.click(nextButton());
    await findContractRow("echo");

    fireEvent.click(prevButton());
    await findContractRow("charlie");
    expect(lastCallCursor()).toBe("cursor-2");
    expect(screen.getByText("Page 2")).toBeDefined();

    fireEvent.click(prevButton());
    await findContractRow("alpha");
    expect(lastCallCursor()).toBeUndefined();
    expect(prevButton().disabled).toBe(true);
  });

  it("returns to the first page when the network changes", async () => {
    await renderPage();
    await findContractRow("alpha");
    fireEvent.click(nextButton());
    await findContractRow("charlie");

    fireEvent.click(screen.getByRole("button", { name: "switch network" }));

    await findContractRow("alpha");
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

// ── Grouped alerts fixtures ─────────────────────────────────────────────────

// Reuse the monitored-contract fixture so the contract filter has an option
// to pick.
const GROUP_CONTRACT_ID = monitored("alpha").contract_id;

function alertGroup(overrides: Partial<AlertGroup> = {}): AlertGroup {
  return {
    id: 1,
    group_key: `${GROUP_CONTRACT_ID}|Critical|`,
    contract_id: GROUP_CONTRACT_ID,
    severity: "Critical",
    rule: "",
    count: 7,
    dedupe_window_secs: 300,
    first_seen: "2026-07-03T08:00:00Z",
    last_seen: "2026-07-03T08:10:00Z",
    last_message: "storage entry expiring",
    backfill_eligible: false,
    ...overrides,
  };
}

function memberAlert(message: string, ledger: number): ContractAlert {
  return {
    contract_id: GROUP_CONTRACT_ID,
    severity: "Critical",
    message,
    ledger,
    tx_hash: `tx-${ledger}`,
    timestamp: "2026-07-03T08:05:00Z",
  };
}

// ── Tests: grouped alerts feed ──────────────────────────────────────────────

describe("WatchdogPage grouped alerts feed", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListMonitoredContracts.mockResolvedValue({
      contracts: [monitored("alpha")],
      next_cursor: "",
    });
    mockListAlertGroups.mockResolvedValue({
      groups: [alertGroup()],
      next_cursor: "",
    });
    mockListAlerts.mockResolvedValue({
      alerts: [
        memberAlert("member one", 120_400),
        memberAlert("member two", 120_401),
      ],
      next_cursor: "",
    });
  });

  afterEach(() => {
    cleanup();
  });

  it("shows the grouped view by default with count and first/last seen", async () => {
    await renderPage();

    await screen.findByText("7");
    expect(
      screen
        .getByRole("button", { name: "Grouped" })
        .getAttribute("aria-pressed")
    ).toBe("true");
    expect(screen.getByText("First seen")).toBeDefined();
    expect(screen.getByText("Last seen")).toBeDefined();
    expect(mockListAlertGroups).toHaveBeenCalledWith({
      contractId: undefined,
      severity: undefined,
      network: undefined,
      limit: 20,
    });
    // The grouped row hides raw messages until the group is expanded.
    expect(screen.queryByText("storage entry expiring")).toBeNull();
  });

  it("expands a group to fetch and list its raw member alerts", async () => {
    await renderPage();
    await screen.findByText("7");

    const toggle = screen.getByRole("button", {
      name: "Expand alert group 1",
    });
    fireEvent.click(toggle);

    await screen.findByText("member one");
    expect(screen.getByText("member two")).toBeDefined();
    expect(mockListAlerts).toHaveBeenCalledWith(GROUP_CONTRACT_ID, {
      severity: "Critical",
      network: undefined,
      limit: 20,
    });
    expect(toggle.getAttribute("aria-expanded")).toBe("true");

    fireEvent.click(
      screen.getByRole("button", { name: "Collapse alert group 1" })
    );
    expect(screen.queryByText("member one")).toBeNull();
  });

  it("filters the grouped feed by severity", async () => {
    await renderPage();
    await screen.findByText("7");

    fireEvent.change(screen.getByLabelText("Severity"), {
      target: { value: "Critical" },
    });

    await waitFor(() => {
      expect(mockListAlertGroups).toHaveBeenLastCalledWith({
        contractId: undefined,
        severity: "Critical",
        network: undefined,
        limit: 20,
      });
    });
  });

  it("filters the grouped feed by contract", async () => {
    await renderPage();
    await screen.findByText("7");

    fireEvent.change(screen.getByLabelText("Contract"), {
      target: { value: GROUP_CONTRACT_ID },
    });

    await waitFor(() => {
      expect(mockListAlertGroups).toHaveBeenLastCalledWith({
        contractId: GROUP_CONTRACT_ID,
        severity: undefined,
        network: undefined,
        limit: 20,
      });
    });
  });

  it("switches to the flat per-event feed", async () => {
    await renderPage();
    await screen.findByText("7");

    fireEvent.click(screen.getByRole("button", { name: "Flat feed" }));

    await screen.findByText("member one");
    expect(
      screen
        .getByRole("button", { name: "Flat feed" })
        .getAttribute("aria-pressed")
    ).toBe("true");
    expect(mockListAlerts).toHaveBeenLastCalledWith(undefined, {
      severity: undefined,
      network: undefined,
      limit: 20,
    });
  });

  it("pages through groups with the returned cursor", async () => {
    mockListAlertGroups
      .mockResolvedValueOnce({
        groups: [alertGroup()],
        next_cursor: "cursor-2",
      })
      .mockResolvedValueOnce({
        groups: [
          alertGroup({
            id: 2,
            group_key: `${GROUP_CONTRACT_ID}|Info|`,
            severity: "Info",
            count: 5,
          }),
        ],
        next_cursor: "",
      });
    await renderPage();
    await screen.findByText("7");

    fireEvent.click(screen.getByRole("button", { name: "Load more groups" }));

    await screen.findByText("5");
    expect(mockListAlertGroups).toHaveBeenLastCalledWith({
      contractId: undefined,
      severity: undefined,
      network: undefined,
      cursor: "cursor-2",
      limit: 20,
    });
  });
});
