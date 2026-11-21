import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { EventInspectorDrawer } from "./EventInspectorDrawer";
import { LocaleProvider } from "@/lib/i18n";
import type { ContractEvent } from "@/lib/types";

const MOCK_EVENT: ContractEvent = {
  id: "evt_inspect_123",
  contract_id: "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
  network: "testnet",
  ledger: 987654,
  ledger_closed_at: "2026-11-20T12:00:00Z",
  tx_hash: "0xdeadbeef12345678",
  type: "contract",
  topic_decoded: ["transfer", "user_1", "user_2"],
  topic_xdr: ["AAAADwAAAAhUcmFuc2Zlcg=="],
  value_decoded: { amount: 500 },
  value_xdr: "AAAABQAAAAAAAAA1",
  in_successful_call: true,
};

function renderDrawer(props: {
  event?: ContractEvent | null;
  isOpen?: boolean;
  onClose?: () => void;
}) {
  const { event = MOCK_EVENT, isOpen = true, onClose = vi.fn() } = props;
  return {
    onClose,
    ...render(
      <LocaleProvider>
        <EventInspectorDrawer event={event} isOpen={isOpen} onClose={onClose} />
      </LocaleProvider>
    ),
  };
}

describe("EventInspectorDrawer", () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it("does not render when isOpen is false or event is null", () => {
    const { rerender } = renderDrawer({ isOpen: false });
    expect(
      screen.queryByTestId("event-inspector-drawer")
    ).not.toBeInTheDocument();

    rerender(
      <LocaleProvider>
        <EventInspectorDrawer event={null} isOpen={true} onClose={vi.fn()} />
      </LocaleProvider>
    );
    expect(
      screen.queryByTestId("event-inspector-drawer")
    ).not.toBeInTheDocument();
  });

  it("renders header with event ID, contract ID, ledger and SUCCESS status", () => {
    renderDrawer({});

    expect(screen.getByText("Event Inspector")).toBeInTheDocument();
    expect(screen.getByText("SUCCESS")).toBeInTheDocument();
    expect(screen.getByText(/evt_inspect_123/)).toBeInTheDocument();
    expect(
      screen.getByTitle(
        "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
      )
    ).toBeInTheDocument();
    expect(screen.getByText("987654")).toBeInTheDocument();
  });

  it("renders topics and value in the Decoded ScVal Tree tab by default", () => {
    renderDrawer({});

    expect(screen.getByText("Decoded ScVal Tree")).toBeInTheDocument();
    expect(screen.getByText("Raw XDR & JSON")).toBeInTheDocument();
    expect(screen.getByText(/Topics \(1\)/i)).toBeInTheDocument();
    expect(screen.getByText(/Event Value/i)).toBeInTheDocument();
  });

  it("switches to Raw XDR & JSON tab and displays raw payloads", () => {
    renderDrawer({});

    const rawTab = screen.getByTestId("tab-raw");
    fireEvent.click(rawTab);

    // Default raw sub-tab is XDR payloads
    expect(screen.getByText("Topic XDR Payloads")).toBeInTheDocument();
    expect(screen.getByText(/AAAADwAAAAhUcmFuc2Zlcg==/)).toBeInTheDocument();
    expect(screen.getByText("AAAABQAAAAAAAAA1")).toBeInTheDocument();

    // Switch to Full Event JSON sub-tab
    const jsonSubTab = screen.getByRole("button", { name: "Event JSON" });
    fireEvent.click(jsonSubTab);

    expect(
      screen.getByRole("button", { name: "Copy Full JSON" })
    ).toBeInTheDocument();
  });

  it("calls onClose when clicking close button", () => {
    const onClose = vi.fn();
    renderDrawer({ onClose });

    const closeBtn = screen.getByLabelText("Close event inspector");
    fireEvent.click(closeBtn);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("calls onClose when pressing Escape", () => {
    const onClose = vi.fn();
    renderDrawer({ onClose });

    fireEvent.keyDown(window, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
