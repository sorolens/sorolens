/**
 * Tests for the /live data feed (issue #139).
 *
 * The hook is exercised through a tiny harness that renders its state, with a
 * short refresh interval so real timers can be used and the polling behaviour
 * is observed rather than asserted about.
 */

import * as matchers from "@testing-library/jest-dom/matchers";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import React from "react";

expect.extend(matchers);

const mockGetRecentEvents = vi.fn();
const mockGetLiveActivity = vi.fn();

vi.mock("@/lib/api", () => ({
  getRecentEvents: (...args: unknown[]) => mockGetRecentEvents(...args),
  getLiveActivity: (...args: unknown[]) => mockGetLiveActivity(...args),
}));

import { TICKER_SIZE, useLiveFeed } from "./useLiveFeed";

function event(id: string, minutesAgo = 0) {
  return {
    id,
    contract_id: "CCONTRACT",
    network: "testnet",
    ledger: 100 + minutesAgo,
    ledger_closed_at: new Date(Date.now() - minutesAgo * 60_000).toISOString(),
    tx_hash: `tx-${id}`,
    type: "contract",
    topic_decoded: ["transfer"],
    topic_xdr: [],
    value_decoded: null,
    value_xdr: "",
    in_successful_call: true,
  };
}

function activity(contractId: string, total: number) {
  return {
    contract_id: contractId,
    label: "",
    network: "testnet",
    total,
    per_minute: [total],
  };
}

function Harness({ refreshMs = 60 }: { refreshMs?: number }) {
  const feed = useLiveFeed(refreshMs, 5);
  return (
    <div>
      <span data-testid="ids">{feed.events.map((e) => e.id).join(",")}</span>
      <span data-testid="totals">
        {feed.contracts.map((c) => `${c.contract_id}:${c.total}`).join(",")}
      </span>
      <span data-testid="new">{[...feed.newIds].join(",")}</span>
      <span data-testid="error">{feed.error ?? ""}</span>
      <span data-testid="loading">{String(feed.loading)}</span>
    </div>
  );
}

beforeEach(() => {
  mockGetRecentEvents.mockReset();
  mockGetLiveActivity.mockReset();
  mockGetRecentEvents.mockResolvedValue({ events: [event("evt_1")] });
  mockGetLiveActivity.mockResolvedValue({
    minutes: 5,
    window_start: new Date().toISOString(),
    contracts: [activity("CA", 1)],
  });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  // The hidden-visibility test replaces this getter; put it back so test
  // order cannot affect the other cases.
  Object.defineProperty(document, "visibilityState", {
    configurable: true,
    get: () => "visible",
  });
});

describe("useLiveFeed", () => {
  it("loads events and activity, then stops loading", async () => {
    render(<Harness />);

    await waitFor(() =>
      expect(screen.getByTestId("ids").textContent).toBe("evt_1")
    );
    expect(screen.getByTestId("totals").textContent).toBe("CA:1");
    expect(screen.getByTestId("loading").textContent).toBe("false");
  });

  it("polls again without the caller doing anything", async () => {
    render(<Harness />);
    await waitFor(() =>
      expect(screen.getByTestId("ids").textContent).toBe("evt_1")
    );

    await waitFor(
      () => expect(mockGetRecentEvents.mock.calls.length).toBeGreaterThan(1),
      { timeout: 2000 }
    );
  });

  it("merges new events newest-first and keeps the existing ones", async () => {
    mockGetRecentEvents
      .mockResolvedValueOnce({ events: [event("evt_1", 5)] })
      .mockResolvedValue({ events: [event("evt_2", 1), event("evt_1", 5)] });

    render(<Harness />);
    await waitFor(() =>
      expect(screen.getByTestId("ids").textContent).toBe("evt_1")
    );

    await waitFor(
      () => expect(screen.getByTestId("ids").textContent).toBe("evt_2,evt_1"),
      { timeout: 2000 }
    );
  });

  it("de-duplicates events that a later poll returns again", async () => {
    mockGetRecentEvents.mockResolvedValue({
      events: [event("evt_1", 1), event("evt_1", 1)],
    });

    render(<Harness />);
    await waitFor(() =>
      expect(screen.getByTestId("ids").textContent).toBe("evt_1")
    );

    // Several polls have happened by now; the list must still hold one row.
    await waitFor(
      () => expect(mockGetRecentEvents.mock.calls.length).toBeGreaterThan(2),
      { timeout: 2000 }
    );
    expect(screen.getByTestId("ids").textContent).toBe("evt_1");
  });

  it("caps the list at TICKER_SIZE", async () => {
    mockGetRecentEvents.mockResolvedValue({
      events: Array.from({ length: TICKER_SIZE + 20 }, (_, i) =>
        event(`evt_${String(i).padStart(3, "0")}`, i)
      ),
    });

    render(<Harness />);
    await waitFor(() =>
      expect(screen.getByTestId("ids").textContent.split(",").length).toBe(
        TICKER_SIZE
      )
    );
  });

  it("flags only the newly arrived ids", async () => {
    mockGetRecentEvents
      .mockResolvedValueOnce({ events: [event("evt_1", 5)] })
      .mockResolvedValue({ events: [event("evt_2", 1), event("evt_1", 5)] });

    render(<Harness />);
    await waitFor(() =>
      expect(screen.getByTestId("ids").textContent).toBe("evt_1")
    );
    // The first load is not "new" — nothing is highlighted on mount.
    expect(screen.getByTestId("new").textContent).toBe("");

    await waitFor(
      () => expect(screen.getByTestId("new").textContent).toBe("evt_2"),
      {
        timeout: 2000,
      }
    );
  });

  it("keeps the last good data and reports the error when a poll fails", async () => {
    mockGetRecentEvents
      .mockResolvedValueOnce({ events: [event("evt_1")] })
      .mockRejectedValue(new Error("upstream down"));

    render(<Harness />);
    await waitFor(() =>
      expect(screen.getByTestId("ids").textContent).toBe("evt_1")
    );

    await waitFor(
      () =>
        expect(screen.getByTestId("error").textContent).toBe("upstream down"),
      { timeout: 2000 }
    );
    // The row survives: a wall display should go stale, not blank.
    expect(screen.getByTestId("ids").textContent).toBe("evt_1");
  });

  it("does not poll while the document is hidden", async () => {
    render(<Harness />);
    await waitFor(() =>
      expect(screen.getByTestId("ids").textContent).toBe("evt_1")
    );

    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      get: () => "hidden",
    });
    document.dispatchEvent(new Event("visibilitychange"));

    const before = mockGetRecentEvents.mock.calls.length;
    await new Promise((r) => setTimeout(r, 300));
    expect(mockGetRecentEvents.mock.calls.length).toBe(before);
  });
});
