/**
 * Tests for apps/web/components/LiveStats.tsx and its client half
 * (issue #627).
 *
 * The first block keeps the original server-render check. The rest exercise
 * the async `LiveStats` server component directly: its data fetch is mocked,
 * and the client half's refresh calls are mocked too, so the tests cover the
 * initial render, a successful refresh, the failure banner, and the retry.
 */

import "@testing-library/jest-dom/vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { LiveStats } from "./LiveStats";
import { LiveStatsClient } from "./LiveStatsClient";
import type { GlobalStats, WatchdogStats } from "@/lib/types";

const getLandingStats = vi.fn();
const getGlobalStats = vi.fn();
const getWatchdogStats = vi.fn();

vi.mock("@/lib/serverData", () => ({
  getLandingStats: (...args: unknown[]) => getLandingStats(...args),
}));

vi.mock("@/lib/api", () => ({
  getGlobalStats: (...args: unknown[]) => getGlobalStats(...args),
  getWatchdogStats: (...args: unknown[]) => getWatchdogStats(...args),
}));

const INITIAL_GLOBAL: GlobalStats = {
  tracked_contracts: 42,
  total_events: 12_345,
  total_invocations: 678,
  total_storage_entries: 90,
};

const INITIAL_WATCHDOG: WatchdogStats = {
  total_monitored: 4,
  healthy: 3,
  degraded: 1,
  unresponsive: 0,
  total_alerts: 2,
  critical_alerts: 1,
};

beforeEach(() => {
  // LiveStatsClient logs the reason for every rejected refresh.
  vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  cleanup();
  getLandingStats.mockReset();
  getGlobalStats.mockReset();
  getWatchdogStats.mockReset();
  vi.restoreAllMocks();
});

describe("server-rendered landing stats", () => {
  it("renders server-provided counters before hydration", () => {
    const markup = renderToStaticMarkup(
      <LiveStatsClient
        initialData={{
          global: INITIAL_GLOBAL,
          watchdog: null,
        }}
      />
    );

    expect(markup).toContain("42");
    expect(markup).toContain("12.3K");
    expect(markup).toContain("Contracts indexed");
  });
});

describe("LiveStats", () => {
  it("renders the server-fetched counters", async () => {
    getLandingStats.mockResolvedValue({
      global: INITIAL_GLOBAL,
      watchdog: INITIAL_WATCHDOG,
    });

    render(await LiveStats({}));

    expect(screen.getByText("42")).toBeVisible();
    expect(screen.getByText("12.3K")).toBeVisible();
    expect(screen.getByText("678")).toBeVisible();
    expect(screen.getByText("Contracts monitored")).toBeVisible();
    expect(screen.getByText("4")).toBeVisible();
  });

  it("refreshes the counters from the API on mount", async () => {
    getLandingStats.mockResolvedValue({
      global: INITIAL_GLOBAL,
      watchdog: INITIAL_WATCHDOG,
    });
    getGlobalStats.mockResolvedValue({
      ...INITIAL_GLOBAL,
      tracked_contracts: 99,
    });
    getWatchdogStats.mockResolvedValue({
      ...INITIAL_WATCHDOG,
      total_monitored: 7,
    });

    render(await LiveStats({}));

    expect(screen.getByText("42")).toBeVisible(); // stale figure first
    await waitFor(() => expect(screen.getByText("99")).toBeVisible());
    expect(screen.getByText("7")).toBeVisible();
    expect(
      screen.queryByText("Couldn't load live stats.")
    ).not.toBeInTheDocument();
  });

  it("keeps the last known figures and shows an error when a refresh fails", async () => {
    getLandingStats.mockResolvedValue({
      global: INITIAL_GLOBAL,
      watchdog: INITIAL_WATCHDOG,
    });
    getGlobalStats.mockRejectedValue(new Error("boom"));
    getWatchdogStats.mockResolvedValue({
      ...INITIAL_WATCHDOG,
      total_monitored: 7,
    });

    render(await LiveStats({}));

    expect(await screen.findByText("Couldn't load live stats.")).toBeVisible();
    // The failed half keeps its stale value; the succeeded half updates.
    expect(screen.getByText("42")).toBeVisible();
    expect(screen.getByText("7")).toBeVisible();
    expect(screen.getByRole("button", { name: "Try again" })).toBeVisible();
  });

  it("recovers through the retry button", async () => {
    getLandingStats.mockResolvedValue({
      global: INITIAL_GLOBAL,
      watchdog: INITIAL_WATCHDOG,
    });
    getGlobalStats.mockRejectedValueOnce(new Error("boom")).mockResolvedValue({
      ...INITIAL_GLOBAL,
      tracked_contracts: 99,
    });
    getWatchdogStats.mockResolvedValue(INITIAL_WATCHDOG);

    render(await LiveStats({}));
    expect(await screen.findByText("Couldn't load live stats.")).toBeVisible();

    fireEvent.click(screen.getByRole("button", { name: "Try again" }));

    await waitFor(() =>
      expect(
        screen.queryByText("Couldn't load live stats.")
      ).not.toBeInTheDocument()
    );
    expect(screen.getByText("99")).toBeVisible();
  });

  it("shows placeholders for every counter when there is no data at all", async () => {
    getLandingStats.mockResolvedValue({ global: null, watchdog: null });
    getGlobalStats.mockRejectedValue(new Error("boom"));
    getWatchdogStats.mockRejectedValue(new Error("boom"));

    render(await LiveStats({}));

    expect(await screen.findByText("Couldn't load live stats.")).toBeVisible();
    expect(screen.getAllByText("—")).toHaveLength(4);
  });

  it("forwards the className to the wrapper", async () => {
    getLandingStats.mockResolvedValue({
      global: INITIAL_GLOBAL,
      watchdog: INITIAL_WATCHDOG,
    });

    const { container } = render(await LiveStats({ className: "my-stats" }));

    expect(container.firstElementChild).toHaveClass("my-stats");
    await act(async () => {}); // settle the client refresh before teardown
  });
});
