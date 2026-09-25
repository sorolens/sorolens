/**
 * Tests for apps/web/components/EventsTable.tsx
 *
 * We use vitest + @testing-library/react + jsdom. The topic/value columns are
 * CSS-truncated, so we assert the full strings are exposed via `title` for
 * hover discoverability.
 */

import * as matchers from "@testing-library/jest-dom/matchers";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { EventsTable } from "./EventsTable";
import type { ContractEvent } from "@/lib/types";

expect.extend(matchers);

const CONTRACT_ID = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGQFW2J";

const event: ContractEvent = {
  id: "evt-1",
  ledger: 42,
  ledger_closed_at: "2026-09-24T00:00:00Z",
  tx_hash: "a".repeat(64),
  type: "contract",
  topic_decoded: [CONTRACT_ID, "transfer"],
  topic_xdr: [],
  value_decoded: CONTRACT_ID,
  value_xdr: "AAAA",
  in_successful_call: true,
};

describe("EventsTable", () => {
  it("exposes the full topic and value in a hover tooltip", () => {
    render(<EventsTable events={[event]} />);

    expect(screen.getAllByTitle(CONTRACT_ID)).toHaveLength(2);
  });
});
