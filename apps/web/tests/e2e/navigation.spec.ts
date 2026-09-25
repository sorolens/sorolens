import { test, expect } from "@playwright/test";
import { expectHeading, mockApi } from "./helpers";

test("navigation links reach the /contracts and /watchdog routes", async ({
  page,
}) => {
  await mockApi(page);
  await page.goto("/contracts");

  // Scope to the header nav: the breadcrumb trail also renders a <nav>.
  const nav = page.locator("header nav");
  await expect(nav).toContainText("Contracts");
  await expect(nav).toContainText("Watchdog");
  await expect(nav).toContainText("Playground");

  await nav.getByRole("link", { name: "Watchdog" }).click();
  await expect(page).toHaveURL(/\/watchdog$/);
  await expectHeading(page, "Watchdog");

  await nav.getByRole("link", { name: "Contracts" }).click();
  await expect(page).toHaveURL(/\/contracts$/);
  await expectHeading(page, "Contracts");

  await nav.getByRole("link", { name: "Playground" }).click();
  await expect(page).toHaveURL(/\/playground$/);
  await expectHeading(page, "API Playground");
});

test("landing page navigation 'Dashboard' link reaches /contracts", async ({
  page,
}) => {
  await mockApi(page);
  await page.goto("/");

  await page.getByRole("link", { name: "Dashboard", exact: true }).click();
  await expect(page).toHaveURL(/\/contracts$/);
  await expectHeading(page, "Contracts");
});
