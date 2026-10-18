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

async function renderStats(
  initialData: { global: unknown; watchdog: unknown } = {
    global: GLOBAL,
    watchdog: WATCHDOG,
  }
) {
  const { LiveStatsClient } = await import("@/components/LiveStatsClient");
  return render(
    <LiveStatsClient
      initialData={
        initialData as Parameters<typeof LiveStatsClient>[0]["initialData"]
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

  it("renders a real zero as 0, not as an error", async () => {
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

  it("keeps the server-rendered figures but still reports a failed refresh", async () => {
    mockGetWatchdogStats.mockRejectedValue(new Error("watchdog down"));
    await renderStats();

    await screen.findByText("Couldn't load live stats.");
    expect(within(tile("Contracts indexed")).getByText("12")).toBeVisible();
    expect(within(tile("Contracts monitored")).getByText("7")).toBeVisible();
  });

  it("retries both requests and clears the error on recovery", async () => {
    mockGetGlobalStats.mockRejectedValueOnce(new Error("down"));
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
