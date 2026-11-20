/**
 * Tests for apps/web/components/StarButton.tsx (issue #765).
 *
 * The button flips local state after the API call resolves, and swallows a
 * failure so the next fetch can correct it — so the negative test asserts the
 * star does *not* flip when the call rejects. `@/lib/api` is mocked.
 */

import "@testing-library/jest-dom/vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { StarButton } from "./StarButton";

const mockAddToWatchlist = vi.fn();
const mockRemoveFromWatchlist = vi.fn();

vi.mock("@/lib/api", () => ({
  addToWatchlist: (...args: unknown[]) => mockAddToWatchlist(...args),
  removeFromWatchlist: (...args: unknown[]) => mockRemoveFromWatchlist(...args),
}));

const CONTRACT_ID = `C${"A".repeat(55)}`;
const USER_ID = "user-1";

function deferred() {
  let resolve!: (value: unknown) => void;
  const promise = new Promise((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

beforeEach(() => {
  vi.clearAllMocks();
  mockAddToWatchlist.mockResolvedValue({ in_watchlist: true });
  mockRemoveFromWatchlist.mockResolvedValue({ in_watchlist: false });
});

afterEach(cleanup);

describe("StarButton", () => {
  it("offers to add when the contract is not watched", () => {
    render(<StarButton contractId={CONTRACT_ID} userId={USER_ID} />);

    const button = screen.getByRole("button", { name: "Add to watchlist" });
    expect(button).toHaveAttribute("title", "Add to watchlist");
    expect(button).toBeEnabled();
  });

  it("offers to remove when the contract is already watched", () => {
    render(
      <StarButton
        contractId={CONTRACT_ID}
        userId={USER_ID}
        initialInWatchlist
      />
    );

    expect(
      screen.getByRole("button", { name: "Remove from watchlist" })
    ).toBeVisible();
  });

  it("adds the contract to the watchlist and flips the label", async () => {
    render(<StarButton contractId={CONTRACT_ID} userId={USER_ID} />);

    fireEvent.click(screen.getByRole("button", { name: "Add to watchlist" }));

    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Remove from watchlist" })
      ).toBeVisible()
    );
    expect(mockAddToWatchlist).toHaveBeenCalledWith(CONTRACT_ID, USER_ID);
    expect(mockRemoveFromWatchlist).not.toHaveBeenCalled();
  });

  it("removes the contract from the watchlist and flips the label", async () => {
    render(
      <StarButton
        contractId={CONTRACT_ID}
        userId={USER_ID}
        initialInWatchlist
      />
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Remove from watchlist" })
    );

    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Add to watchlist" })
      ).toBeVisible()
    );
    expect(mockRemoveFromWatchlist).toHaveBeenCalledWith(CONTRACT_ID, USER_ID);
    expect(mockAddToWatchlist).not.toHaveBeenCalled();
  });

  it("keeps the star unchanged when adding fails", async () => {
    mockAddToWatchlist.mockRejectedValue(new Error("401 Unauthorized"));
    render(<StarButton contractId={CONTRACT_ID} userId={USER_ID} />);

    fireEvent.click(screen.getByRole("button", { name: "Add to watchlist" }));

    await waitFor(() => expect(mockAddToWatchlist).toHaveBeenCalledTimes(1));
    // The optimistic update must not claim success after a rejection.
    expect(
      screen.getByRole("button", { name: "Add to watchlist" })
    ).toBeVisible();
    expect(
      screen.queryByRole("button", { name: "Remove from watchlist" })
    ).toBeNull();
  });

  it("keeps the star unchanged when removing fails", async () => {
    mockRemoveFromWatchlist.mockRejectedValue(new Error("401 Unauthorized"));
    render(
      <StarButton
        contractId={CONTRACT_ID}
        userId={USER_ID}
        initialInWatchlist
      />
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Remove from watchlist" })
    );

    await waitFor(() =>
      expect(mockRemoveFromWatchlist).toHaveBeenCalledTimes(1)
    );
    expect(
      screen.getByRole("button", { name: "Remove from watchlist" })
    ).toBeVisible();
  });

  it("disables itself while the request is pending", async () => {
    const pending = deferred();
    mockAddToWatchlist.mockReturnValue(pending.promise);
    render(<StarButton contractId={CONTRACT_ID} userId={USER_ID} />);

    fireEvent.click(screen.getByRole("button", { name: "Add to watchlist" }));

    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Add to watchlist" })
      ).toBeDisabled()
    );

    pending.resolve({ in_watchlist: true });

    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Remove from watchlist" })
      ).toBeEnabled()
    );
  });
});
