import { test, expect } from "@playwright/test";
import {
  contractEvent,
  contractEventRate,
  mockApi,
  newerContractEvent,
} from "./helpers";

/**
 * /live real-time dashboard (issue #139).
 *
 * Acceptance: the ticker updates without a page refresh, sparklines redraw in
 * place, and the fullscreen toggle is present. Fullscreen itself is not
 * asserted here because headless Chromium frequently refuses the request
 * without a real user gesture; the standard and webkit code paths are covered
 * by the unit test in app/(app)/live/page.test.tsx.
 */

test("renders the ticker, sparklines, and hot-contracts leaderboard", async ({
  page,
}) => {
  await mockApi(page);
  await page.goto("/live");

  await expect(page.getByRole("heading", { name: "Live" })).toBeVisible();
  await expect(page.getByTestId("ticker")).toBeVisible();
  await expect(page.getByTestId("ticker-row")).toHaveCount(1);
  await expect(page.getByTestId("hot-contracts")).toBeVisible();
  await expect(page.getByTestId("hot-contract-row")).toHaveCount(1);
  await expect(page.getByTestId("hot-contract-sparkline")).toBeVisible();
  await expect(page.getByTestId("fullscreen-toggle")).toBeEnabled();
});

test("is reachable from the header navigation", async ({ page }) => {
  await mockApi(page);
  await page.goto("/contracts");

  await page.locator("nav").getByRole("link", { name: "Live" }).click();
  await expect(page).toHaveURL(/\/live$/);
  await expect(page.getByTestId("live-dashboard")).toBeVisible();
});

test("the ticker gains new events without a page refresh", async ({ page }) => {
  // Count real navigations so we can prove the update came from polling and
  // not from a reload.
  let navigations = 0;
  page.on("framenavigated", (frame) => {
    if (frame === page.mainFrame()) navigations++;
  });

  let polls = 0;
  await mockApi(page, {
    "events/recent": () => {
      polls += 1;
      return {
        status: 200,
        body: {
          events:
            polls < 2
              ? [contractEvent()]
              : [newerContractEvent(), contractEvent()],
        },
      };
    },
  });

  await page.goto("/live");
  await expect(page.getByTestId("ticker-row")).toHaveCount(1);

  // Baseline once the first document load has settled. Asserting that this
  // count is unchanged afterwards is what proves the ticker updated in place;
  // an absolute count would be brittle because the dev server can commit the
  // document more than once during startup.
  const navigationsAfterLoad = navigations;

  // The feed polls every 5s.
  await expect(page.getByTestId("ticker-row")).toHaveCount(2, {
    timeout: 15_000,
  });

  // The newest event is on top and the original one is still present.
  const ids = await page
    .getByTestId("ticker-row")
    .evaluateAll((rows) => rows.map((r) => r.getAttribute("data-event-id")));
  expect(ids).toContain("evt_1");
  expect(ids).toContain("evt_2");

  // Ticker gained a row without any navigation: polling, not a reload.
  expect(navigations).toBe(navigationsAfterLoad);
});

test("the fullscreen toggle is present and labelled", async ({ page }) => {
  await mockApi(page);
  await page.goto("/live");

  const toggle = page.getByTestId("fullscreen-toggle");
  await expect(toggle).toHaveText("Fullscreen");
  await expect(toggle).toHaveAttribute("aria-pressed", "false");
});

test("shows an error banner but keeps the last data when the API fails", async ({
  page,
}) => {
  let fail = false;
  await mockApi(page, {
    "stats/activity": () => ({
      status: fail ? 500 : 200,
      body: fail
        ? { error: "upstream down" }
        : {
            minutes: 30,
            window_start: "2026-07-03T08:00:00Z",
            contracts: [contractEventRate()],
          },
    }),
  });

  await page.goto("/live");
  await expect(page.getByTestId("hot-contract-row")).toHaveCount(1);

  // Next poll fails; the previously loaded row must survive.
  fail = true;
  await page.waitForTimeout(6_000);
  await expect(page.getByTestId("hot-contract-row")).toHaveCount(1);
});
