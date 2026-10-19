import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useEventStream } from "@/hooks/useEventStream";
import { getRecentEvents } from "@/lib/api";

vi.mock("@/lib/api", () => ({
  getContractEvents: vi.fn(),
  getRecentEvents: vi.fn(),
}));

class MockEventSource {
  static instances: MockEventSource[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: (() => void) | null = null;
  close = vi.fn();

  constructor(public url: string) {
    MockEventSource.instances.push(this);
  }
}

describe("useEventStream", () => {
  beforeEach(() => {
    MockEventSource.instances = [];
    vi.mocked(getRecentEvents).mockResolvedValue({ events: [] });
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("polls the global recent-events endpoint without a contract filter", async () => {
    vi.useFakeTimers();
    vi.stubGlobal("EventSource", undefined);

    const { result } = renderHook(() =>
      useEventStream({ pollFallbackInterval: 1000 })
    );
    expect(result.current.status).toBe("polling");

    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });

    expect(getRecentEvents).toHaveBeenCalledWith(50);
  });

  it("includes the last received event ID when creating a new stream", () => {
    vi.stubGlobal("EventSource", MockEventSource);
    const { rerender } = renderHook(
      ({ enabled }) => useEventStream({ enabled }),
      { initialProps: { enabled: true } }
    );

    act(() => {
      MockEventSource.instances[0].onmessage?.({
        data: JSON.stringify({
          type: "event",
          event: { id: "evt-42" },
        }),
        lastEventId: "evt-42",
      } as MessageEvent);
    });
    rerender({ enabled: false });
    rerender({ enabled: true });

    expect(MockEventSource.instances[1].url).toContain("last_event_id=evt-42");
  });
});
