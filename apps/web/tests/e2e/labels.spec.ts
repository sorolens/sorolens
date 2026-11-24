import { expect, test } from "@playwright/test";
import { CONTRACT_ID, expectHeading, mockApi } from "./helpers";

/**
 * End-to-end coverage for the label registry (issue #590): browse the seeded
 * label, filter by scope, create one through the API, and reject invalid input.
 */

test.describe("Label registry", () => {
  test.beforeEach(async ({ page }) => {
    await mockApi(page);
    await page.goto("/labels");
    await expectHeading(page, "Label registry");
    await expect(page.getByTestId("label-row")).toHaveCount(1);
  });

  test("lists labels and filters them by scope", async ({ page }) => {
    await expect(page.getByTestId("label-row")).toContainText("Stellar DEX");
    await expect(page.getByTestId("label-row")).toContainText("Public");

    await page.getByLabel("Filter by scope").selectOption("workspace");
    await expect(page.getByText("No labels found")).toBeVisible();
  });

  test("creates a workspace label through the API", async ({ page }) => {
    await page.getByLabel("Name").fill("Treasury");
    await page.getByLabel("Account or contract ID").fill(CONTRACT_ID);
    await page.getByRole("button", { name: "Save label" }).click();

    await expect(page.getByText("Label saved.")).toBeVisible();
    await expect(page.getByTestId("label-row").first()).toContainText(
      "Treasury"
    );
    await expect(page.getByTestId("label-row").first()).toContainText(
      "Workspace"
    );
  });

  test("validates the contract id before calling the API", async ({ page }) => {
    await page.getByLabel("Name").fill("Treasury");
    await page.getByLabel("Account or contract ID").fill("not-an-id");
    await page.getByRole("button", { name: "Save label" }).click();

    await expect(page.getByText(/56-character Stellar account/)).toBeVisible();
    await expect(page.getByText("Label saved.")).toHaveCount(0);
  });

  test("is reachable from the header navigation", async ({ page }) => {
    await page
      .getByRole("navigation", { name: "Primary" })
      .getByRole("link", {
        name: "Labels",
      })
      .click();
    await expect(page).toHaveURL(/\/labels$/);
  });
});
