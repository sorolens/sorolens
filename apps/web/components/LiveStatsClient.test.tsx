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

const mockGetGlobalStats = vi.fn();
const mockGetWatchdogStats = vi.fn();

vi.mock("@/lib/api", () => ({
  getGlobalStats: (...args: unknown[]) => mockGetGlobalStats(...args),
  getWatchdogStats: (...args: unknown[]) => mockGetWatchdogStats(...args),
}));

expect.extend(matchers);

const GLOBAL = {
  tracked_contracts: 12,
  total_events: 3400,
  total_invocations: 5600,
  total_storage_entries: 1,
};
const WATCHDOG = {
  total_monitored: 7,
  healthy: 7,
  degraded: 0,
  unresponsive: 0,
  total_alerts: 0,
  critical_alerts: 0,
};

function tile(label: string): HTMLElement {
  return screen.getByText(label).parentElement as HTMLElement;
}

/** The age stamped on one tile, e.g. "5m ago". Only present when stale. */
function ageOf(label: string): HTMLElement {
  return screen.getByTestId(`live-stats-age-${label}`);
}

async function renderStats(
  initialData: { global: unknown; watchdog: unknown } = {
    global: GLOBAL,
    watchdog: WATCHDOG,
  },
  generatedAt?: number
) {
  const { LiveStatsClient } = await import("@/components/LiveStatsClient");
  return render(
    <LiveStatsClient
      initialData={
        {
          ...initialData,
          ...(generatedAt === undefined ? {} : { generatedAt }),
        } as Parameters<typeof LiveStatsClient>[0]["initialData"]
      }
    />
  );
}

describe("LiveStatsClient", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(console, "error").mockImplementation(() => {});
    mockGetGlobalStats.mockResolvedValue(GLOBAL);
    mockGetWatchdogStats.mockResolvedValue(WATCHDOG);
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it("renders the counts with no error when the refresh succeeds", async () => {
    await renderStats();

    await waitFor(() => expect(mockGetWatchdogStats).toHaveBeenCalled());
    expect(within(tile("Contracts indexed")).getByText("12")).toBeVisible();
    expect(within(tile("Events tracked")).getByText("3.4K")).toBeVisible();
    expect(within(tile("Contracts monitored")).getByText("7")).toBeVisible();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("shows a loading state, never a placeholder zero, while the first figures are in flight", async () => {
    mockGetGlobalStats.mockReturnValue(new Promise(() => {}));
    mockGetWatchdogStats.mockReturnValue(new Promise(() => {}));
    await renderStats({ global: null, watchdog: null });

    const grid = screen.getByRole("status", { name: "Loading live stats" });
    expect(grid.getAttribute("aria-busy")).toBe("true");
    for (const label of [
      "Contracts indexed",
      "Events tracked",
      "Invocations traced",
      "Contracts monitored",
    ]) {
      expect(within(grid).getByText(`Loading ${label}`)).toBeInTheDocument();
    }
    expect(screen.queryByText("—")).toBeNull();
    expect(screen.queryByText("0")).toBeNull();
    expect(screen.queryByText("Couldn't load live stats.")).toBeNull();
  });

  it("shows a figure the API returned as zero, not an error", async () => {
    mockGetGlobalStats.mockResolvedValue({ ...GLOBAL, total_events: 0 });
    await renderStats({
      global: { ...GLOBAL, total_events: 0 },
      watchdog: WATCHDOG,
    });

    await waitFor(() => expect(mockGetGlobalStats).toHaveBeenCalled());
    expect(within(tile("Events tracked")).getByText("0")).toBeVisible();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("shows an error and dashes, not zeros, when there is no data and the refresh fails", async () => {
    mockGetGlobalStats.mockRejectedValue(new Error("down"));
    mockGetWatchdogStats.mockRejectedValue(new Error("down"));
    await renderStats({ global: null, watchdog: null });

    await screen.findByText("Couldn't load live stats.");
    for (const label of [
      "Contracts indexed",
      "Events tracked",
      "Invocations traced",
      "Contracts monitored",
    ]) {
      expect(within(tile(label)).getByText("—")).toBeVisible();
      expect(within(tile(label)).queryByText("0")).toBeNull();
    }
  });

  it("keeps the last known figures and labels the stale tile with its age", async () => {
    mockGetWatchdogStats.mockRejectedValue(new Error("watchdog down"));
    await renderStats(
      { global: GLOBAL, watchdog: WATCHDOG },
      Date.now() - 5 * 60_000
    );

    await screen.findByText("Some figures are out of date.");
    expect(within(tile("Contracts indexed")).getByText("12")).toBeVisible();
    expect(within(tile("Contracts monitored")).getByText("7")).toBeVisible();
    // Only the source whose refresh failed is marked, and it says how old it is.
    expect(ageOf("Contracts monitored").textContent).toBe("5m ago");
    expect(within(tile("Contracts indexed")).queryByText("5m ago")).toBeNull();
    // A failed refresh is still an error state: no red banner claiming the
    // numbers are current, and a way to try again.
    expect(screen.getByRole("button", { name: "Try again" })).toBeVisible();
  });

  it("labels a cached figure as stale once it is past the freshness window", async () => {
    // Nothing new has arrived yet, so what is on screen is the cached read.
    mockGetGlobalStats.mockReturnValue(new Promise(() => {}));
    mockGetWatchdogStats.mockReturnValue(new Promise(() => {}));
    await renderStats(
      { global: GLOBAL, watchdog: WATCHDOG },
      Date.now() - 10 * 60_000
    );

    await screen.findByText("Some figures are out of date.");
    expect(ageOf("Contracts indexed").textContent).toBe("10m ago");
    expect(ageOf("Contracts monitored").textContent).toBe("10m ago");
    expect(screen.queryByText("Couldn't load live stats.")).toBeNull();
  });

  it("clears the stale labels when a retry succeeds", async () => {
    mockGetWatchdogStats.mockRejectedValueOnce(new Error("down"));
    await renderStats({ global: GLOBAL, watchdog: WATCHDOG });

    await screen.findByText("Some figures are out of date.");
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));

    await waitFor(() =>
      expect(screen.queryByText("Some figures are out of date.")).toBeNull()
    );
    expect(within(tile("Contracts monitored")).getByText("7")).toBeVisible();
  });

  it("retries both requests and clears the error on recovery", async () => {
    mockGetGlobalStats.mockRejectedValueOnce(new Error("down"));
    mockGetWatchdogStats.mockRejectedValueOnce(new Error("down"));
    await renderStats({ global: null, watchdog: null });

    await screen.findByText("Couldn't load live stats.");
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));

    await waitFor(() =>
      expect(within(tile("Contracts indexed")).getByText("12")).toBeVisible()
    );
    expect(screen.queryByRole("alert")).toBeNull();
    expect(mockGetGlobalStats).toHaveBeenCalledTimes(2);
    expect(mockGetWatchdogStats).toHaveBeenCalledTimes(2);
  });
});
