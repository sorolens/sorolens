/**
 * Tests for apps/web/app/(app)/watchlist/page.tsx: a failed request must show
 * an error with a retry, not the "No bookmarks yet" empty state.
 */

import * as matchers from "@testing-library/jest-dom/matchers";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
  }: {
    children: React.ReactNode;
    href: string;
  }) => <a href={href}>{children}</a>,
}));

vi.mock("@/lib/useSession", () => ({
  useSessionUserId: () => "user-1",
}));

vi.mock("@/components/StarButton", () => ({
  StarButton: ({ contractId }: { contractId: string }) => (
    <button type="button">star {contractId}</button>
  ),
}));

const mockListWatchlist = vi.fn();

vi.mock("@/lib/api", () => ({
  listWatchlist: (...args: unknown[]) => mockListWatchlist(...args),
  addToWatchlist: vi.fn(),
  removeFromWatchlist: vi.fn(),
}));

expect.extend(matchers);

const ITEM = {
  contract_id: "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
  added_at: "2026-09-01T12:00:00Z",
};

async function renderPage() {
  const { default: WatchlistPage } = await import("@/app/(app)/watchlist/page");
  return render(<WatchlistPage />);
}

describe("WatchlistPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListWatchlist.mockResolvedValue({ items: [ITEM] });
  });

  afterEach(() => {
    cleanup();
  });

  it("lists bookmarked contracts", async () => {
    await renderPage();

    expect(await screen.findByText(ITEM.contract_id)).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("shows the empty state when the request succeeds with no items", async () => {
    mockListWatchlist.mockResolvedValue({ items: [] });
    await renderPage();

    expect(await screen.findByText("No bookmarks yet")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("shows an error, not the empty state, when the API fails", async () => {
    mockListWatchlist.mockRejectedValue(new Error("API down"));
    await renderPage();

    expect(
      await screen.findByText("Couldn't load your watchlist.")
    ).toBeInTheDocument();
    expect(screen.getByText("API down")).toBeInTheDocument();
    expect(screen.queryByText("No bookmarks yet")).toBeNull();
  });

  it("retries and shows the list once the API recovers", async () => {
    mockListWatchlist.mockRejectedValueOnce(new Error("API down"));
    await renderPage();

    await screen.findByText("Couldn't load your watchlist.");
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));

    expect(await screen.findByText(ITEM.contract_id)).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
    expect(mockListWatchlist).toHaveBeenCalledTimes(2);
  });
});
