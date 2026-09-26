/**
 * Tests for apps/web/app/(app)/live/page.tsx — the real-time dashboard
 * (issue #139).
 *
 * Recharts is mocked: ResponsiveContainer measures its parent and jsdom has no
 * layout, so the real chart would render nothing measurable. The mock keeps
 * the component tree and the props the page passes intact, which is what these
 * tests assert on.
 */

import * as matchers from "@testing-library/jest-dom/matchers";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import React from "react";

expect.extend(matchers);

// ── Mocks ───────────────────────────────────────────────────────────────────

vi.mock("recharts", () => ({
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="responsive-container">{children}</div>
  ),
  LineChart: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="line-chart">{children}</div>
  ),
  Line: ({ stroke }: { stroke?: string }) => (
    <div data-testid="line" data-stroke={stroke} />
  ),
}));

const mockGetRecentEvents = vi.fn();
const mockGetLiveActivity = vi.fn();

vi.mock("@/lib/api", () => ({
  getRecentEvents: (...args: unknown[]) => mockGetRecentEvents(...args),
  getLiveActivity: (...args: unknown[]) => mockGetLiveActivity(...args),
}));

import LivePage from "./page";

const CONTRACT_ID =
  "CAVRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C33";

function liveEvent(id: string) {
  return {
    id,
    contract_id: CONTRACT_ID,
    network: "testnet",
    ledger: 120_400,
    ledger_closed_at: "2026-07-03T08:00:00Z",
    tx_hash: "a1b2c3",
    type: "contract",
    topic_decoded: ["transfer"],
    topic_xdr: ["AAAAAA=="],
    value_decoded: null,
    value_xdr: "",
    in_successful_call: true,
  };
}

function liveActivity() {
  return {
    minutes: 30,
    window_start: "2026-07-03T08:00:00Z",
    contracts: [
      {
        contract_id: CONTRACT_ID,
        label: "Escrow DEX",
        network: "testnet",
        total: 6,
        per_minute: [...Array(29).fill(0), 6],
      },
    ],
  };
}

// ── Fullscreen stubs ────────────────────────────────────────────────────────

const requestFullscreen = vi.fn(async () => {});
const exitFullscreen = vi.fn(async () => {});
const webkitRequestFullscreen = vi.fn(async () => {});
const webkitExitFullscreen = vi.fn(async () => {});

/** What `document.fullscreenElement` reports; tests flip this. */
let fullscreenElement: Element | null = null;

function defineProp(target: object, key: string, value: unknown) {
  Object.defineProperty(target, key, {
    configurable: true,
    writable: true,
    value,
  });
}

beforeEach(() => {
  mockGetRecentEvents.mockReset();
  mockGetLiveActivity.mockReset();
  mockGetRecentEvents.mockResolvedValue({ events: [liveEvent("evt_1")] });
  mockGetLiveActivity.mockResolvedValue(liveActivity());

  requestFullscreen.mockClear();
  exitFullscreen.mockClear();
  webkitRequestFullscreen.mockClear();
  webkitExitFullscreen.mockClear();
  fullscreenElement = null;

  defineProp(HTMLElement.prototype, "requestFullscreen", requestFullscreen);
  defineProp(
    HTMLElement.prototype,
    "webkitRequestFullscreen",
    webkitRequestFullscreen
  );
  defineProp(document, "exitFullscreen", exitFullscreen);
  defineProp(document, "webkitExitFullscreen", webkitExitFullscreen);
  Object.defineProperty(document, "fullscreenElement", {
    configurable: true,
    get: () => fullscreenElement,
  });
  Object.defineProperty(document, "webkitFullscreenElement", {
    configurable: true,
    get: () => null,
  });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

// ── Rendering ───────────────────────────────────────────────────────────────

describe("LivePage", () => {
  it("renders the ticker, the leaderboard, and a sparkline per contract", async () => {
    render(<LivePage />);

    await waitFor(() => expect(screen.getByTestId("ticker")).toBeDefined());
    expect(screen.getAllByTestId("ticker-row")).toHaveLength(1);
    expect(screen.getByTestId("hot-contracts")).toBeDefined();
    expect(screen.getAllByTestId("hot-contract-row")).toHaveLength(1);
    expect(screen.getAllByTestId("hot-contract-sparkline")).toHaveLength(1);
  });

  it("keeps the sparkline mounted so it repaints instead of flickering", async () => {
    render(<LivePage />);
    await waitFor(() =>
      expect(screen.getAllByTestId("hot-contract-sparkline")).toHaveLength(1)
    );

    const before = screen.getAllByTestId("hot-contract-sparkline")[0];
    expect(screen.getByTestId("line-chart")).toBeDefined();

    // A manual refresh re-fetches but must not unmount the chart: the same DOM
    // node is still in place, which is what avoids a full re-render flicker.
    fireEvent.click(screen.getByTestId("live-refresh"));
    await waitFor(() =>
      expect(mockGetLiveActivity.mock.calls.length).toBeGreaterThan(1)
    );

    const after = screen.getAllByTestId("hot-contract-sparkline")[0];
    expect(after).toBe(before);
  });

  it("shows an empty ticker message when there are no events", async () => {
    mockGetRecentEvents.mockResolvedValue({ events: [] });
    render(<LivePage />);

    await waitFor(() =>
      expect(screen.getByTestId("ticker-empty")).toBeDefined()
    );
  });

  it("surfaces an error banner when the feed fails", async () => {
    mockGetRecentEvents.mockRejectedValue(new Error("boom"));
    render(<LivePage />);

    await waitFor(() => expect(screen.getByTestId("live-error")).toBeDefined());
    expect(screen.getByTestId("live-error").textContent).toContain("boom");
  });

  // ── Fullscreen (Chrome and Safari) ────────────────────────────────────────

  it("requests fullscreen through the standard API", async () => {
    render(<LivePage />);
    await waitFor(() => expect(screen.getByTestId("ticker")).toBeDefined());

    await act(async () => {
      fireEvent.click(screen.getByTestId("fullscreen-toggle"));
    });

    expect(requestFullscreen).toHaveBeenCalledTimes(1);
    expect(webkitRequestFullscreen).not.toHaveBeenCalled();
  });

  it("exits fullscreen through the standard API and reflects the state", async () => {
    render(<LivePage />);
    await waitFor(() => expect(screen.getByTestId("ticker")).toBeDefined());

    // Simulate the browser entering fullscreen and reporting it.
    const container = screen.getByTestId("live-dashboard");
    fullscreenElement = container;
    await act(async () => {
      document.dispatchEvent(new Event("fullscreenchange"));
    });

    expect(screen.getByTestId("fullscreen-toggle").textContent).toBe(
      "Exit fullscreen"
    );

    await act(async () => {
      fireEvent.click(screen.getByTestId("fullscreen-toggle"));
    });
    expect(exitFullscreen).toHaveBeenCalledTimes(1);
  });

  it("falls back to the webkit API when the standard one is missing (Safari)", async () => {
    // Safari historically exposed only the prefixed variants.
    delete (HTMLElement.prototype as { requestFullscreen?: unknown })
      .requestFullscreen;

    render(<LivePage />);
    await waitFor(() => expect(screen.getByTestId("ticker")).toBeDefined());

    await act(async () => {
      fireEvent.click(screen.getByTestId("fullscreen-toggle"));
    });

    expect(webkitRequestFullscreen).toHaveBeenCalledTimes(1);
    expect(requestFullscreen).not.toHaveBeenCalled();
  });

  it("falls back to webkitExitFullscreen when exiting (Safari)", async () => {
    delete (HTMLElement.prototype as { requestFullscreen?: unknown })
      .requestFullscreen;
    delete (document as unknown as { exitFullscreen?: unknown }).exitFullscreen;

    render(<LivePage />);
    await waitFor(() => expect(screen.getByTestId("ticker")).toBeDefined());

    fullscreenElement = screen.getByTestId("live-dashboard");
    await act(async () => {
      document.dispatchEvent(new Event("webkitfullscreenchange"));
    });

    await act(async () => {
      fireEvent.click(screen.getByTestId("fullscreen-toggle"));
    });

    expect(webkitExitFullscreen).toHaveBeenCalledTimes(1);
    expect(exitFullscreen).not.toHaveBeenCalled();
  });

  it("reports an error when the browser supports neither API", async () => {
    delete (HTMLElement.prototype as { requestFullscreen?: unknown })
      .requestFullscreen;
    delete (HTMLElement.prototype as { webkitRequestFullscreen?: unknown })
      .webkitRequestFullscreen;

    render(<LivePage />);
    await waitFor(() => expect(screen.getByTestId("ticker")).toBeDefined());

    await act(async () => {
      fireEvent.click(screen.getByTestId("fullscreen-toggle"));
    });

    await waitFor(() =>
      expect(screen.getByTestId("live-error").textContent).toContain(
        "does not support fullscreen"
      )
    );
  });
});
