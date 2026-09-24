import { test, expect } from "@playwright/test";
import {
  blockApi,
  expectHeading,
  mockApi,
  emptyWatchdogHandlers,
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

test("/watchdog shows the empty state when the API is unavailable", async ({
  page,
}) => {
  await blockApi(page);
  await page.goto("/watchdog");

  await expectHeading(page, "Watchdog");
  await expect(page.getByText("No monitored contracts yet")).toBeVisible();
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
