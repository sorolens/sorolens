import { test, expect } from "@playwright/test";
import { expectHeading, mockApi } from "./helpers";

test("landing page renders the hero section", async ({ page }) => {
  await mockApi(page);
  await page.goto("/");

  await expectHeading(
    page,
    "Real-time monitoring and on-chain health checks for Soroban contracts."
  );
  await expect(
    page.getByRole("link", { name: "View Dashboard" })
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Copy CLI quickstart" })
  ).toBeVisible();
  await expect(
    page.getByRole("img", { name: /Sorolens contracts dashboard/ })
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "API Playground" })
  ).toHaveAttribute("href", "/playground");
  await expect(
    page.getByRole("link", { name: "Compare contracts" })
  ).toHaveAttribute("href", "/compare");
  await expect(
    page.getByRole("link", { name: "Health reports" })
  ).toHaveAttribute("href", "/watchdog/reports");
});

test("landing page 'View Dashboard' navigates to /contracts", async ({
  page,
}) => {
  await mockApi(page);
  await page.goto("/");

  await page.getByRole("link", { name: "View Dashboard" }).click();
  await expect(page).toHaveURL(/\/contracts$/);
  await expectHeading(page, "Contracts");
});

test("getting started docs are linked from the site and render the repo guide", async ({
  page,
}) => {
  await mockApi(page);
  await page.goto("/");
  await page
    .getByRole("navigation", { name: "Main navigation" })
    .getByRole("link", { name: "Docs" })
    .click();

  await expect(page).toHaveURL(/\/docs$/);
  await expectHeading(page, "Getting started with Sorolens");
  await expectHeading(page, "Run Sorolens locally");
  await expect(
    page.getByRole("link", { name: "OpenAPI specification" })
  ).toHaveAttribute(
    "href",
    "https://github.com/sorolens/sorolens/blob/main/docs/openapi.yaml"
  );
  await expect(
    page
      .getByRole("navigation", { name: "Footer navigation" })
      .getByRole("link", { name: "Docs" })
  ).toBeVisible();
});
