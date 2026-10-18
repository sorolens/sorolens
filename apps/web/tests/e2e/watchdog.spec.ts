import { test, expect } from "@playwright/test";
import {
  alertGroup,
  blockApi,
  contractAlert,
  expectHeading,
  mockApi,
  emptyWatchdogHandlers,
  monitoredContract,
} from "./helpers";

test("/watchdog shows empty states when there is no data yet", async ({
  page,
}) => {
  await mockApi(page, emptyWatchdogHandlers());
  await page.goto("/watchdog");

  await expectHeading(page, "Watchdog");
  await expect(page.getByText("No monitored contracts yet")).toBeVisible();
  await expect(page.getByText("No alerts yet")).toBeVisible();
});

test("/watchdog shows errors with a retry when the API is unavailable", async ({
  page,
}) => {
  await blockApi(page);
  await page.goto("/watchdog");

  await expectHeading(page, "Watchdog");
  await expect(
    page.getByText("Couldn't load monitored contracts.")
  ).toBeVisible();
  await expect(page.getByText("Couldn't load watchdog stats.")).toBeVisible();
  await expect(page.getByText("No monitored contracts yet")).toBeHidden();
});

test("/watchdog renders monitored contracts when the API returns data", async ({
  page,
}) => {
  await mockApi(page);
  await page.goto("/watchdog");

  await expectHeading(page, "Watchdog");
  const table = page.getByRole("table");
  await expect(table.getByText("Escrow DEX")).toBeVisible();
  await expect(table.getByText("Healthy")).toBeVisible();
  await expect(table.getByText("60s")).toBeVisible();
  await expect(page.getByText("Monitored", { exact: true })).toBeVisible();
});

test("/watchdog shows grouped alerts by default with expandable members", async ({
  page,
}) => {
  let lastAlertsUrl = "";
  await mockApi(page, {
    "api/v1/alerts": (route) => {
      lastAlertsUrl = route.request().url();
      return { status: 200, body: { groups: [alertGroup()], next_cursor: "" } };
    },
    "watchdog/alerts": () => ({
      status: 200,
      body: { alerts: [contractAlert("member alert one")], next_cursor: "" },
    }),
    // Expanding a group fetches the group's raw members through the
    // per-contract alerts feed; other contract-detail suffixes keep the
    // monitored-contract default.
    "watchdog/contracts/": (route) => {
      const path = new URL(route.request().url()).pathname;
      if (path.endsWith("/alerts")) {
        return {
          status: 200,
          body: {
            alerts: [contractAlert("member alert one")],
            next_cursor: "",
          },
        };
      }
      return { status: 200, body: monitoredContract() };
    },
  });
  await page.goto("/watchdog");

  // The grouped feed is the default view and shows count + first/last seen.
  await expect(page.getByRole("button", { name: "Grouped" })).toHaveAttribute(
    "aria-pressed",
    "true"
  );
  await expect(
    page.getByRole("button", { name: "Expand alert group 1" })
  ).toBeVisible();
  await expect(
    page.getByRole("cell", { name: "3", exact: true })
  ).toBeVisible();
  await expect(page.getByText("First seen")).toBeVisible();
  await expect(page.getByText("Last seen")).toBeVisible();

  // Expanding the group fetches and lists its raw member alerts.
  await page.getByRole("button", { name: "Expand alert group 1" }).click();
  await expect(page.getByText("member alert one")).toBeVisible();

  // The severity filter reaches the grouped feed as ?severity=.
  await page.getByLabel("Severity").selectOption("Critical");
  await expect
    .poll(() => new URL(lastAlertsUrl).searchParams.get("severity"))
    .toBe("Critical");
});
