import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ContractEvent } from "@/lib/types";
import { appendStreamEvent, MAX_BUFFERED_EVENTS } from "./useEventStream";

function makeEvent(id: string): ContractEvent {
  return {
    id,
    contract_id: "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
    network: "testnet",
    ledger: 1,
    ledger_closed_at: "2026-01-01T00:00:00Z",
    tx_hash: id,
    type: "contract",
    topic_decoded: null,
    topic_xdr: [],
    value_decoded: null,
    value_xdr: "",
    in_successful_call: true,
  };
}

describe("live event stream buffer", () => {
  it("keeps only the latest 500 unique events and counts omitted events", () => {
    let buffer = { events: [] as ContractEvent[], droppedCount: 0 };
    for (let index = 0; index < MAX_BUFFERED_EVENTS + 25; index += 1) {
      buffer = appendStreamEvent(buffer, makeEvent(`evt-${index}`));
    }

    expect(buffer.events).toHaveLength(MAX_BUFFERED_EVENTS);
    expect(buffer.events[0].id).toBe("evt-524");
    expect(buffer.events.at(-1)?.id).toBe("evt-25");
    expect(buffer.droppedCount).toBe(25);
  });

  it("does not count duplicate events as dropped", () => {
    const event = makeEvent("evt-1");
    const buffer = { events: [event], droppedCount: 0 };

    expect(appendStreamEvent(buffer, event)).toBe(buffer);
  });
});

// ---------------------------------------------------------------------------
// Hook behaviour (issue #616): Last-Event-ID resume, catch-up on reconnect and
// a global polling fallback.
// ---------------------------------------------------------------------------

import { act, renderHook } from "@testing-library/react";
import { useEventStream } from "./useEventStream";

const getContractEvents = vi.fn();
const listAllEvents = vi.fn();
vi.mock("@/lib/api", () => ({
  getContractEvents: (...args: unknown[]) => getContractEvents(...args),
  listAllEvents: (...args: unknown[]) => listAllEvents(...args),
}));

/** Scriptable stand-in for the browser's EventSource. */
class FakeEventSource {
  static instances: FakeEventSource[] = [];
  url: string;
  onopen: (() => void) | null = null;
  onmessage: ((e: { data: string }) => void) | null = null;
  onerror: (() => void) | null = null;
  closed = false;

  constructor(url: string) {
    this.url = url;
    FakeEventSource.instances.push(this);
  }
  close() {
    this.closed = true;
  }
  emitOpen() {
    this.onopen?.();
  }
  emitEvent(event: ContractEvent) {
    this.onmessage?.({
      data: JSON.stringify({ type: "event", event }),
    });
  }
  emitError() {
    this.onerror?.();
  }
}

function lastSource(): FakeEventSource {
  return FakeEventSource.instances.at(-1) as FakeEventSource;
}

function resetModules() {
  getContractEvents.mockReset();
  listAllEvents.mockReset();
  FakeEventSource.instances = [];
}

beforeEach(() => {
  resetModules();
  vi.stubGlobal("EventSource", FakeEventSource as unknown as typeof EventSource);
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

describe("useEventStream", () => {
  it("does not send last_event_id on a first connection", () => {
    renderHook(() => useEventStream({ contractId: "C1" }));
    expect(lastSource().url).not.toContain("last_event_id");
    expect(lastSource().url).toContain("contract_id=C1");
  });

  it("sends last_event_id when reconnecting after receiving events", async () => {
    vi.useFakeTimers();
    renderHook(() => useEventStream({ contractId: "C1" }));

    const first = lastSource();
    act(() => {
      first.emitOpen();
      first.emitEvent(makeEvent("evt-1"));
      first.emitError();
    });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1000);
    });

    expect(first.closed).toBe(true);
    expect(lastSource()).not.toBe(first);
    expect(lastSource().url).toContain("last_event_id=evt-1");
  });

  it("fills the gap with a catch-up fetch after a reconnect", async () => {
    vi.useFakeTimers();
    const { result } = renderHook(() => useEventStream({ contractId: "C1" }));

    const first = lastSource();
    act(() => {
      first.emitOpen();
      first.emitEvent(makeEvent("evt-1"));
      first.emitError();
    });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1000);
    });

    // The catch-up returns newer events plus one already seen; the buffer
    // must gain only the new ones.
    getContractEvents.mockResolvedValue({
      events: [makeEvent("evt-2"), makeEvent("evt-1")],
    });
    await act(async () => {
      lastSource().emitOpen();
      // Drain the catch-up promise under fake timers.
      await vi.advanceTimersByTimeAsync(0);
    });

    expect(getContractEvents).toHaveBeenCalledWith("C1", { limit: 50 });
    expect(result.current.events.map((e) => e.id)).toEqual(["evt-2", "evt-1"]);
  });

  it("falls back to global polling when the stream fails without a contractId", async () => {
    vi.useFakeTimers();
    const { result } = renderHook(() => useEventStream({}));

    listAllEvents.mockResolvedValue({
      events: [makeEvent("g-1")],
    });

    // Three consecutive failures on the (re)connected source: polling
    // takes over, and without a contract_id it must poll the
    // cross-contract feed, not silently do nothing. The third failure
    // switches immediately, so only microtasks need flushing there.
    for (const advance of [1000, 2000, 0]) {
      act(() => {
        lastSource().emitError();
      });
      await act(async () => {
        await vi.advanceTimersByTimeAsync(advance);
      });
    }

    expect(result.current.status).toBe("polling");
    expect(listAllEvents).toHaveBeenCalledWith({ limit: 50 });
    // Flush the in-flight poll promise (waitFor does not cooperate with
    // fake timers).
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(result.current.events.map((e) => e.id)).toContain("g-1");
  });

  it("reports the status and keeps events received over the stream", async () => {
    const { result } = renderHook(() => useEventStream({ contractId: "C1" }));

    expect(result.current.status).toBe("connecting");
    act(() => {
      lastSource().emitOpen();
      lastSource().emitEvent(makeEvent("evt-9"));
    });
    expect(result.current.status).toBe("connected");
    expect(result.current.events.map((e) => e.id)).toEqual(["evt-9"]);
    expect(result.current.lastEventAt).not.toBeNull();
  });
});

