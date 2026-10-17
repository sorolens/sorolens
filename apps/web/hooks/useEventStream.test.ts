import { describe, expect, it } from "vitest";
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
