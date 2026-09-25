import { test, expect } from "@playwright/test";
import { CONTRACT_ID, expectHeading, mockApi } from "./helpers";

test("breadcrumbs on a nested route link up the hierarchy", async ({ page }) => {
  await mockApi(page);
  await page.goto(`/contracts/${CONTRACT_ID}`);

  const breadcrumbs = page.getByRole("navigation", { name: "Breadcrumb" });

  await expect(breadcrumbs.getByRole("link", { name: "Home" })).toHaveAttribute(
    "href",
    "/"
  );
  await expect(
    breadcrumbs.getByRole("link", { name: "Contracts" })
  ).toHaveAttribute("href", "/contracts");
  await expect(breadcrumbs.locator('a[aria-current="page"]')).toHaveAttribute(
    "href",
    `/contracts/${CONTRACT_ID}`
  );

  // Each crumb navigates: the Contracts crumb leads back to the list.
  await breadcrumbs.getByRole("link", { name: "Contracts" }).click();
  await expect(page).toHaveURL(/\/contracts$/);
  await expectHeading(page, "Contracts");
});

test("breadcrumbs render on the watchdog detail route", async ({ page }) => {
  await mockApi(page);
  await page.goto("/watchdog/1");

  const breadcrumbs = page.getByRole("navigation", { name: "Breadcrumb" });
  await expect(
    breadcrumbs.getByRole("link", { name: "Watchdog" })
  ).toHaveAttribute("href", "/watchdog");
  await expect(
    breadcrumbs.getByRole("link", { name: "Home" })
  ).toHaveAttribute("href", "/");
});
