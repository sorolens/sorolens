/**
 * Tests for apps/web/components/EventsTable.tsx (issue #183: copy JSON button).
 *
 * We use vitest + @testing-library/react + jsdom.
 * @sorolens/ui resolves to its source via the vitest alias, so the real
 * CopyButton is exercised. The clipboard is mocked.
 */

// Registers jest-dom matchers with vitest, including their types.
import "@testing-library/jest-dom/vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { EventsTable } from "./EventsTable";
import { LocaleProvider } from "@/lib/i18n";
import type { ContractEvent } from "@/lib/types";

const EVENT: ContractEvent = {
  id: "evt_1",
  contract_id: "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
  network: "testnet",
  ledger: 120_400,
  ledger_closed_at: "2026-07-03T08:00:00Z",
  tx_hash: "a1b2c3",
  type: "transfer",
  topic_decoded: [
    "transfer",
    "GD2E2SJSD2C5SQGMH2YPIXI6F7NS2S6MZ2G4T3VDWXQH2FQT4M6URPLT",
  ],
  topic_xdr: ["AAAAAA=="],
  value_decoded: { amount: 1000 },
  value_xdr: "AAAAAw==",
  in_successful_call: true,
};

function renderEventsTable(events: ContractEvent[]) {
  return render(
    <LocaleProvider>
      <EventsTable events={events} />
    </LocaleProvider>
  );
}

describe("EventsTable copy JSON button", () => {
  const writeText = vi.fn().mockResolvedValue(undefined);

  beforeEach(() => {
    writeText.mockClear();
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it("copies the fully-decoded event as pretty-printed JSON", async () => {
    renderEventsTable([EVENT]);

    const button = screen.getByRole("button", {
      name: `Copy JSON for event ${EVENT.id}`,
    });
    fireEvent.click(button);

    // JSON.stringify(decoded event, null, 2) per the issue's guidance.
    await waitFor(() =>
      expect(writeText).toHaveBeenCalledWith(JSON.stringify(EVENT, null, 2))
    );
  });

  it("renders one copy button per event row", () => {
    renderEventsTable([EVENT, { ...EVENT, id: "evt_2" }]);

    expect(
      screen.getAllByRole("button", { name: /^Copy JSON for event / })
    ).toHaveLength(2);
  });

  it("virtualizes a 10k-event result set", () => {
    const events = Array.from({ length: 10_000 }, (_, index) => ({
      ...EVENT,
      id: `evt_${index}`,
      ledger: index,
    }));
    const { container } = renderEventsTable(events);

    expect(
      container.querySelectorAll('tbody tr:not([aria-hidden="true"])').length
    ).toBeLessThan(100);
    expect(
      screen.queryByRole("button", {
        name: "Copy JSON for event evt_9999",
      })
    ).toBeNull();
  });

  it("does not expand the row when the copy button is clicked", async () => {
    renderEventsTable([EVENT]);

    fireEvent.click(
      screen.getByRole("button", { name: `Copy JSON for event ${EVENT.id}` })
    );
    await waitFor(() => expect(writeText).toHaveBeenCalled());

    // The expanded details panel only renders when the row is toggled open.
    expect(screen.queryByText("Decoded Topics")).not.toBeInTheDocument();
  });

  it("still expands the row when the row itself is clicked", () => {
    renderEventsTable([EVENT]);

    // The ledger cell is unique and has no click handler of its own, so a
    // click on it bubbles up to the row toggle. ("transfer" appears in both
    // the Type and Topic cells, so it is ambiguous.)
    fireEvent.click(screen.getByText(String(EVENT.ledger)));
    expect(screen.getByText("Decoded Topics")).toBeVisible();
  });
});
