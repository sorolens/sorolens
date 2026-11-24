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
});

test("landing page shows product screenshots and copyable quickstarts", async ({
  page,
}) => {
  await mockApi(page);
  await page.goto("/");

  await expect(page.getByTestId("landing-screenshot")).toHaveCount(3);
  await expect(
    page.getByRole("button", { name: "Copy CLI quickstart" })
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Copy API request" })
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Open the API playground" })
  ).toHaveAttribute("href", "/playground");
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
