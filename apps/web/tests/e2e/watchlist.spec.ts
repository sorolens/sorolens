import { expect, test, type Route } from "@playwright/test";
import { CONTRACT_ID, expectHeading, mockApi, watchlistItem } from "./helpers";

/**
 * End-to-end coverage for `/watchlist` (issue #626).
 *
 * The page is identity-scoped: it mints a workspace session through the
 * mocked `POST /api/v1/auth/session`, then lists the caller's bookmarks from
 * `GET /api/v1/watchlist`. One handler branches on the HTTP method so the
 * list and the star-toggle DELETE share a single override keyed on
 * "watchlist".
 */

type WatchlistItems = Array<Record<string, unknown>>;

/** Method-aware /api/v1/watchlist handler over a mutable items list. */
function watchlistHandlers(
  getItems: () => WatchlistItems,
  onRemove?: () => void
) {
  return {
    watchlist: (route: Route) => {
      const method = route.request().method();
      if (method === "DELETE") {
        onRemove?.();
        return { status: 200, body: { in_watchlist: false } };
      }
      if (method === "POST") {
        return { status: 201, body: { in_watchlist: true } };
      }
      return { status: 200, body: { items: getItems() } };
    },
  };
}

test("/watchlist lists the caller's bookmarked contracts", async ({ page }) => {
  await mockApi(
    page,
    watchlistHandlers(() => [watchlistItem()])
  );
  await page.goto("/watchlist");
  await expectHeading(page, "Watchlist");

  const table = page.getByRole("table");
  await expect(table.getByText("Contract")).toBeVisible();
  await expect(table.getByText("Added")).toBeVisible();
  // The bookmark links through to the contract detail page.
  await expect(
    table.getByRole("link", { name: CONTRACT_ID, exact: true })
  ).toHaveAttribute("href", `/contracts/${CONTRACT_ID}`);
  await expect(
    table.getByRole("button", { name: "Remove from watchlist" })
  ).toBeVisible();
});

test("/watchlist shows an empty state when nothing is bookmarked", async ({
  page,
}) => {
  await mockApi(
    page,
    watchlistHandlers(() => [])
  );
  await page.goto("/watchlist");
  await expectHeading(page, "Watchlist");

  await expect(page.getByText("No bookmarks yet")).toBeVisible();
  await expect(
    page.getByText(
      "Star a contract on the Contracts page to add it to your watchlist."
    )
  ).toBeVisible();
  await expect(page.getByRole("table")).toHaveCount(0);
});

test("/watchlist shows an error with a retry when the API fails", async ({
  page,
}) => {
  // The first list request fails; the retry through the error state succeeds.
  let calls = 0;
  await mockApi(page, {
    watchlist: (route: Route) => {
      if (route.request().method() === "DELETE") {
        return { status: 200, body: { in_watchlist: false } };
      }
      calls += 1;
      if (calls === 1) {
        return { status: 500, body: { error: "boom" } };
      }
      return { status: 200, body: { items: [watchlistItem()] } };
    },
  });
  await page.goto("/watchlist");
  await expectHeading(page, "Watchlist");

  await expect(page.getByText("Couldn't load your watchlist.")).toBeVisible();
  await expect(page.getByText("boom")).toBeVisible();
  await expect(page.getByText("No bookmarks yet")).toBeHidden();

  await page.getByRole("button", { name: "Try again" }).click();
  await expect(page.getByRole("table")).toBeVisible();
  await expect(page.getByText("Couldn't load your watchlist.")).toBeHidden();
});

test("toggling the star removes the bookmark from the API", async ({
  page,
}) => {
  let removed = 0;
  await mockApi(
    page,
    watchlistHandlers(
      () => [watchlistItem()],
      () => {
        removed += 1;
      }
    )
  );
  await page.goto("/watchlist");
  await expectHeading(page, "Watchlist");

  const star = page.getByRole("button", { name: "Remove from watchlist" });
  await expect(star).toBeVisible();
  await star.click();

  // StarButton flips its own state once the DELETE succeeds.
  await expect(
    page.getByRole("button", { name: "Add to watchlist" })
  ).toBeVisible();
  await expect.poll(() => removed).toBe(1);
});
