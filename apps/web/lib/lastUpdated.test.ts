/**
 * Unit tests for apps/web/lib/lastUpdated.ts plus the `fetchJson` wiring in
 * apps/web/lib/api.ts that records timestamps after successful requests.
 */

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  formatClockTime,
  formatRelativeTime,
  getLastUpdated,
  getServerLastUpdated,
  recordLastUpdated,
  resetLastUpdated,
  resourceFromUrl,
  subscribeLastUpdated,
} from "./lastUpdated";
import { getWatchdogStats, listContracts } from "./api";

const T0 = Date.UTC(2026, 0, 2, 3, 4, 5);

describe("lastUpdated store", () => {
  beforeEach(() => {
    resetLastUpdated();
  });

  afterEach(() => {
    resetLastUpdated();
  });

  it("starts with no recorded fetch", () => {
    expect(getLastUpdated()).toEqual({ timestamp: null, resource: null });
  });

  it("reports an always-empty server snapshot so hydration is stable", () => {
    recordLastUpdated("contracts", T0);
    expect(getServerLastUpdated()).toEqual({ timestamp: null, resource: null });
    // The client snapshot still has the real value.
    expect(getLastUpdated()).toEqual({ timestamp: T0, resource: "contracts" });
  });

  it("keeps only the most recent fetch", () => {
    recordLastUpdated("contracts", T0);
    recordLastUpdated("watchdog", T0 + 1_000);
    expect(getLastUpdated()).toEqual({
      timestamp: T0 + 1_000,
      resource: "watchdog",
    });
  });

  it("notifies subscribers on change and ignores no-op updates", () => {
    const listener = vi.fn();
    const unsubscribe = subscribeLastUpdated(listener);

    recordLastUpdated("contracts", T0);
    expect(listener).toHaveBeenCalledTimes(1);

    // Same resource and timestamp: nothing changed, no re-render.
    recordLastUpdated("contracts", T0);
    expect(listener).toHaveBeenCalledTimes(1);

    unsubscribe();
    recordLastUpdated("events", T0 + 1);
    expect(listener).toHaveBeenCalledTimes(1);
  });

  it("clears the timestamp on reset", () => {
    recordLastUpdated("contracts", T0);
    resetLastUpdated();
    expect(getLastUpdated()).toEqual({ timestamp: null, resource: null });
  });
});

describe("formatRelativeTime", () => {
  it("returns an empty string for non-finite input", () => {
    expect(formatRelativeTime(Number.NaN, T0)).toBe("");
  });

  const relativeCases: Array<[number, string]> = [
    [0, "just now"],
    [10_000, "just now"],
    [59_999, "just now"],
    [60_000, "1m ago"],
    [5 * 60_000, "5m ago"],
    [60 * 60_000, "1h ago"],
    [23 * 60 * 60_000, "23h ago"],
    [24 * 60 * 60_000, "1d ago"],
    [3 * 24 * 60 * 60_000, "3d ago"],
  ];

  it.each(relativeCases)(
    "formats %i ms of elapsed time as %s",
    (elapsed, expected) => {
      expect(formatRelativeTime(T0 - elapsed, T0)).toBe(expected);
    }
  );

  it("never reports future timestamps as negative", () => {
    expect(formatRelativeTime(T0 + 60_000, T0)).toBe("just now");
  });
});

describe("formatClockTime", () => {
  it("returns a locale clock label", () => {
    const label = formatClockTime(T0, "en-GB");
    expect(label).toMatch(/^\d{2}:\d{2}:\d{2}$/);
  });

  it("returns an empty string for non-finite input", () => {
    expect(formatClockTime(Number.NaN)).toBe("");
  });
});

describe("resourceFromUrl", () => {
  const resourceCases: Array<[string, string]> = [
    ["http://localhost:8080/api/v1/contracts?limit=20", "contracts"],
    ["http://localhost:8080/api/v1/contracts/CABC/stats?window=7d", "stats"],
    ["http://localhost:8080/api/v1/contracts/CABC/events", "events"],
    ["http://localhost:8080/api/v1/watchdog/stats", "watchdog"],
    ["http://localhost:8080/api/v1/watchdog/alerts?limit=20", "watchdog"],
    ["http://localhost:8080/api/v1/watchlist", "watchlist"],
    ["http://localhost:8080/api/v1/stats/global", "global stats"],
    ["http://localhost:8080/api/v1/unknown", "dashboard"],
  ];

  it.each(resourceCases)("maps %s to %s", (url, expected) => {
    expect(resourceFromUrl(url)).toBe(expected);
  });
});

describe("fetchJson integration", () => {
  const fetchMock = vi.fn();

  beforeEach(() => {
    resetLastUpdated();
    fetchMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    resetLastUpdated();
  });

  it("records a timestamp after a successful contracts fetch", async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({ contracts: [], cursor: null, has_more: false }),
    });

    await listContracts();

    const { timestamp, resource } = getLastUpdated();
    expect(resource).toBe("contracts");
    expect(typeof timestamp).toBe("number");
    expect(timestamp).toBeGreaterThan(0);
  });

  it("labels watchdog fetches", async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({ total_monitored: 0 }),
    });

    await getWatchdogStats();

    expect(getLastUpdated().resource).toBe("watchdog");
  });

  it("does not record failed requests", async () => {
    fetchMock.mockResolvedValue({
      ok: false,
      status: 500,
      statusText: "Server Error",
      json: async () => ({ error: "boom" }),
    });

    await expect(listContracts()).rejects.toThrow("boom");
    expect(getLastUpdated().timestamp).toBeNull();
  });
});
