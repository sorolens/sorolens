import "@testing-library/jest-dom/vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LocaleProvider } from "@/lib/i18n";

const streamMocks = vi.hoisted(() => ({ useEventStream: vi.fn() }));
vi.mock("@/hooks/useEventStream", () => ({
  useEventStream: streamMocks.useEventStream,
}));
vi.mock("next/link", () => ({
  default: ({
    children,
    href,
  }: {
    children: React.ReactNode;
    href: string;
  }) => <a href={href}>{children}</a>,
}));

import EventsPage from "./page";

describe("Live events controls", () => {
  beforeEach(() => {
    streamMocks.useEventStream.mockReturnValue({
      events: [],
      clearEvents: vi.fn(),
      droppedCount: 12,
      status: "connected",
      isConnected: true,
      isPolling: false,
      lastEventAt: null,
    });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("shows omitted-event count and pauses/resumes ingestion", async () => {
    render(
      <LocaleProvider>
        <EventsPage />
      </LocaleProvider>
    );

    expect(screen.getByTestId("live-buffer-status")).toHaveTextContent(
      "Showing latest 0 events · 12 older events omitted"
    );
    fireEvent.click(screen.getByTestId("live-stream-pause"));
    await waitFor(() =>
      expect(streamMocks.useEventStream).toHaveBeenLastCalledWith(
        expect.objectContaining({ enabled: false })
      )
    );
    expect(screen.getByTestId("live-stream-pause")).toHaveTextContent("Resume");

    fireEvent.click(screen.getByTestId("live-stream-pause"));
    await waitFor(() =>
      expect(streamMocks.useEventStream).toHaveBeenLastCalledWith(
        expect.objectContaining({ enabled: true })
      )
    );
  });
});
