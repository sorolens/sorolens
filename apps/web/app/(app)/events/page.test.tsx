/**
 * Tests for apps/web/app/(app)/events/page.tsx, the global events explorer
 * (issue #97): pagination, filters, and empty/error states.
 */

import "@testing-library/jest-dom/vitest";
import {
  cleanup,
  fireEvent,
  render as renderWithoutLocale,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { GlobalEvent, GlobalEventsResponse } from "@/lib/types";
import { LocaleProvider } from "@/lib/i18n";
import type { ReactNode } from "react";

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
  }: {
    children: React.ReactNode;
    href: string;
  }) => <a href={href}>{children}</a>,
}));

vi.mock("@/lib/network", () => ({
  useNetwork: () => ({ network: "all" }),
  networkFilter: () => undefined,
}));

const { searchParams, routerPush, routerReplace } = vi.hoisted(() => {
  const searchParams = new URLSearchParams();
  // The mocked router applies navigations to the shared searchParams, the
  // way the real one updates the URL, so a test can simulate Back by
  // reverting the params and re-rendering.
  const applyTo = (url: string) => {
    const queryIndex = url.indexOf("?");
    const qs = queryIndex >= 0 ? url.slice(queryIndex + 1) : "";
    for (const key of Array.from(searchParams.keys())) searchParams.delete(key);
    for (const [key, value] of new URLSearchParams(qs)) {
      searchParams.set(key, value);
    }
  };
  const routerPush = vi.fn((url: string) => applyTo(url));
  const routerReplace = vi.fn((url: string) => applyTo(url));
  return { searchParams, routerPush, routerReplace };
});

vi.mock("next/navigation", () => ({
  useSearchParams: () => searchParams,
  useRouter: () => ({
    push: routerPush,
    replace: routerReplace,
    back: vi.fn(),
  }),
  usePathname: () => "/events",
}));

const listAllEvents =
  vi.fn<(params: Record<string, unknown>) => Promise<GlobalEventsResponse>>();
vi.mock("@/lib/api", () => ({
  listAllEvents: (params: Record<string, unknown>) => listAllEvents(params),
}));

import EventsExplorerPage from "./page";

function resetQuery() {
  for (const key of Array.from(searchParams.keys())) searchParams.delete(key);
  routerPush.mockClear();
  routerReplace.mockClear();
}

function render(ui: ReactNode) {
  return renderWithoutLocale(<LocaleProvider>{ui}</LocaleProvider>);
}

function event(n: number, overrides: Partial<GlobalEvent> = {}): GlobalEvent {
  return {
    id: `evt-${n}`,
    contract_id: `CAAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQD${String(n).padStart(3, "0")}`,
    network: "testnet",
    ledger: 1000 + n,
    ledger_closed_at: "2026-09-25T10:00:00Z",
    tx_hash: `tx${n}`,
    type: "contract",
    topic_decoded: ["transfer"],
    topic_xdr: [],
    value_decoded: { amount: n },
    value_xdr: "",
    in_successful_call: true,
    ...overrides,
  };
}

beforeEach(() => {
  listAllEvents.mockReset();
  resetQuery();
});

afterEach(() => {
  cleanup();
  resetQuery();
});

describe("EventsExplorerPage", () => {
  it("renders a page of events and pages forward and back", async () => {
    listAllEvents.mockImplementation(async (params) =>
      params.cursor === "c2"
        ? { events: [event(3)], next_cursor: "" }
        : { events: [event(1), event(2)], next_cursor: "c2" }
    );
    render(<EventsExplorerPage />);

    expect(await screen.findByText("1,001")).toBeInTheDocument();
    expect(screen.getByText("1,002")).toBeInTheDocument();
    expect(screen.getByText("Page 1")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "← Previous" })).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Next →" }));
    expect(await screen.findByText("1,003")).toBeInTheDocument();
    expect(screen.getByText("Page 2")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Next →" })).toBeDisabled();
    expect(listAllEvents).toHaveBeenLastCalledWith(
      expect.objectContaining({ cursor: "c2", limit: 25 })
    );

    fireEvent.click(screen.getByRole("button", { name: "← Previous" }));
    expect(await screen.findByText("1,001")).toBeInTheDocument();
    expect(screen.getByText("Page 1")).toBeInTheDocument();
    expect(listAllEvents).toHaveBeenLastCalledWith(
      expect.objectContaining({ cursor: undefined })
    );
  });

  it("expands a row to show the decoded data", async () => {
    listAllEvents.mockResolvedValue({ events: [event(7)], next_cursor: "" });
    render(<EventsExplorerPage />);

    const toggle = await screen.findByRole("button", { expanded: false });
    fireEvent.click(toggle);
    expect(screen.getByRole("button", { expanded: true })).toBeInTheDocument();
    expect(
      screen.getByText(/"amount": 7/, { selector: "pre" })
    ).toBeInTheDocument();
  });

  it("sends the contract, type and date filters and resets to page 1", async () => {
    listAllEvents.mockResolvedValue({ events: [event(1)], next_cursor: "c2" });
    render(<EventsExplorerPage />);
    await screen.findByText("1,001");

    fireEvent.change(screen.getByLabelText("Contract ID"), {
      target: { value: " CAAQ " },
    });
    fireEvent.change(screen.getByLabelText("Event type"), {
      target: { value: "system" },
    });
    fireEvent.change(screen.getByLabelText("From (UTC)"), {
      target: { value: "2026-09-01" },
    });
    fireEvent.change(screen.getByLabelText("To (UTC)"), {
      target: { value: "2026-09-02" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Apply" }));

    await waitFor(() =>
      expect(listAllEvents).toHaveBeenLastCalledWith(
        expect.objectContaining({
          contractId: "CAAQ",
          type: "system",
          since: "2026-09-01T00:00:00Z",
          until: "2026-09-02T23:59:59Z",
          cursor: undefined,
        })
      )
    );
  });

  it("blocks an inverted date range", async () => {
    listAllEvents.mockResolvedValue({ events: [], next_cursor: "" });
    render(<EventsExplorerPage />);
    await screen.findByText(/No events indexed yet/);

    fireEvent.change(screen.getByLabelText("From (UTC)"), {
      target: { value: "2026-09-05" },
    });
    fireEvent.change(screen.getByLabelText("To (UTC)"), {
      target: { value: "2026-09-01" },
    });
    expect(screen.getByRole("alert")).toHaveTextContent(/start date/);
    expect(screen.getByRole("button", { name: "Apply" })).toBeDisabled();
  });

  it("shows the empty state when nothing is indexed", async () => {
    listAllEvents.mockResolvedValue({ events: [], next_cursor: "" });
    render(<EventsExplorerPage />);

    const empty = await screen.findByText(/No events indexed yet/);
    expect(empty).toHaveTextContent(
      "No events indexed yet. Track a contract to start seeing events here."
    );
    expect(
      screen.getByRole("link", { name: "Track a contract" })
    ).toHaveAttribute("href", "/contracts");
    expect(
      screen.queryByRole("navigation", { name: "Pagination" })
    ).not.toBeInTheDocument();
  });

  it("distinguishes 'no matches' from 'nothing indexed'", async () => {
    listAllEvents.mockResolvedValueOnce({
      events: [event(1)],
      next_cursor: "",
    });
    listAllEvents.mockResolvedValue({ events: [], next_cursor: "" });
    render(<EventsExplorerPage />);
    await screen.findByText("1,001");

    fireEvent.change(screen.getByLabelText("Contract ID"), {
      target: { value: "CZZZ" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Apply" }));
    expect(
      await screen.findByText("No events match these filters.")
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Clear filters" }));
    await waitFor(() =>
      expect(listAllEvents).toHaveBeenLastCalledWith(
        expect.objectContaining({ contractId: undefined })
      )
    );
  });

  it("shows an error with a retry", async () => {
    listAllEvents.mockRejectedValueOnce(new Error("API down"));
    listAllEvents.mockResolvedValue({ events: [event(1)], next_cursor: "" });
    render(<EventsExplorerPage />);

    expect(await screen.findByText("API down")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByText("1,001")).toBeInTheDocument();
  });

  it("wraps the table in a horizontal scroller for small screens", async () => {
    listAllEvents.mockResolvedValue({ events: [event(1)], next_cursor: "" });
    render(<EventsExplorerPage />);
    await screen.findByText("1,001");
    expect(screen.getByRole("table").parentElement).toHaveClass(
      "overflow-x-auto"
    );
  });

  // ---- URL-synced state (issue #618) --------------------------------------

  it("restores the filters and page from the URL on first render", async () => {
    listAllEvents.mockResolvedValue({ events: [event(1)], next_cursor: "" });
    searchParams.set("contract", "CAAQ");
    searchParams.set("type", "system");
    searchParams.set("from", "2026-09-01");
    searchParams.set("to", "2026-09-02");
    searchParams.set("cursor", "cur-3");
    searchParams.set("page", "3");

    render(<EventsExplorerPage />);

    await waitFor(() =>
      expect(listAllEvents).toHaveBeenCalledWith(
        expect.objectContaining({
          contractId: "CAAQ",
          type: "system",
          since: "2026-09-01T00:00:00Z",
          until: "2026-09-02T23:59:59Z",
          cursor: "cur-3",
          limit: 25,
        })
      )
    );
    expect(screen.getByText("Page 3")).toBeInTheDocument();
    // The form fields show the restored filters too.
    expect(screen.getByLabelText("Contract ID")).toHaveValue("CAAQ");
    expect(screen.getByLabelText("Event type")).toHaveValue("system");
  });

  it("pushes applied filters into the URL", async () => {
    listAllEvents.mockResolvedValue({ events: [event(1)], next_cursor: "c2" });
    render(<EventsExplorerPage />);
    await screen.findByText("1,001");

    fireEvent.change(screen.getByLabelText("Contract ID"), {
      target: { value: "CAAQ" },
    });
    fireEvent.change(screen.getByLabelText("Event type"), {
      target: { value: "system" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Apply" }));

    await waitFor(() =>
      expect(listAllEvents).toHaveBeenLastCalledWith(
        expect.objectContaining({ contractId: "CAAQ", type: "system" })
      )
    );
    expect(routerPush).toHaveBeenCalledWith("/events?contract=CAAQ&type=system", {
      scroll: false,
    });
  });

  it("pushes the cursor and page number when paging forward", async () => {
    listAllEvents.mockResolvedValue({
      events: [event(1), event(2)],
      next_cursor: "c2",
    });
    render(<EventsExplorerPage />);
    await screen.findByText("1,001");

    fireEvent.click(screen.getByRole("button", { name: "Next →" }));
    await waitFor(() =>
      expect(listAllEvents).toHaveBeenLastCalledWith(
        expect.objectContaining({ cursor: "c2" })
      )
    );
    expect(routerPush).toHaveBeenLastCalledWith("/events?cursor=c2&page=2", {
      scroll: false,
    });
  });

  it("restores the previous view when the query string changes (back/forward)", async () => {
    listAllEvents.mockResolvedValue({ events: [event(1)], next_cursor: "" });
    const view = render(<EventsExplorerPage />);
    await screen.findByText("1,001");

    fireEvent.change(screen.getByLabelText("Contract ID"), {
      target: { value: "CZZZ" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Apply" }));
    await waitFor(() =>
      expect(listAllEvents).toHaveBeenLastCalledWith(
        expect.objectContaining({ contractId: "CZZZ" })
      )
    );

    // Simulate browser Back: the query string reverts and the component
    // re-renders with the previous params.
    searchParams.delete("contract");
    view.rerender(
      <LocaleProvider>
        <EventsExplorerPage />
      </LocaleProvider>
    );

    await waitFor(() =>
      expect(listAllEvents).toHaveBeenLastCalledWith(
        expect.objectContaining({ contractId: undefined })
      )
    );
    expect(screen.getByLabelText("Contract ID")).toHaveValue("");
  });

  it("drops malformed URL values back to defaults", async () => {
    listAllEvents.mockResolvedValue({ events: [event(1)], next_cursor: "" });
    searchParams.set("type", "nonsense");
    searchParams.set("page", "zero");
    searchParams.set("from", "not-a-date");

    render(<EventsExplorerPage />);

    await screen.findByText("1,001");
    expect(listAllEvents).toHaveBeenCalledWith(
      expect.objectContaining({ type: undefined, cursor: undefined })
    );
    expect(screen.getByText("Page 1")).toBeInTheDocument();
    expect(screen.getByLabelText("From (UTC)")).toHaveValue("");
  });
});
