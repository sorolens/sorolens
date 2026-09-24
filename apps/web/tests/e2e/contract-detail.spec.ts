import { test, expect } from "@playwright/test";
import { CONTRACT_ID, mockApi, expectHeading } from "./helpers";

test("contract detail renders header, stats and activity sections", async ({
  page,
}) => {
  await mockApi(page);
  await page.goto(`/contracts/${CONTRACT_ID}`);

  await expectHeading(page, CONTRACT_ID.slice(0, 8), { exact: false });
  await expect(page.getByText("Escrow DEX")).toBeVisible();
  await expect(page.getByText("active", { exact: true })).toBeVisible();

  await expect(page.getByRole("heading", { name: "Activity" })).toBeVisible();
  await expect(page.getByText("Event Volume")).toBeVisible();
  await expect(page.getByText("Invocations").first()).toBeVisible();
});

test("contract detail renders Events and Storage TTL sections", async ({
  page,
}) => {
  await mockApi(page);
  await page.goto(`/contracts/${CONTRACT_ID}`);

  await expect(page.getByRole("heading", { name: "Events" })).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Storage TTL" })
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Snapshot / replay" })
  ).toBeVisible();
  await expect(
    page.getByText("transfer", { exact: false }).first()
  ).toBeVisible();
  await expect(
    page.getByText("balance", { exact: false }).first()
  ).toBeVisible();
});

test("contract detail links back to the contracts list", async ({ page }) => {
  await mockApi(page);
  await page.goto(`/contracts/${CONTRACT_ID}`);

  await page
    .getByRole("banner")
    .getByRole("link", { name: "Contracts" })
    .click();
  await expect(page).toHaveURL(/\/contracts$/);
  await expectHeading(page, "Contracts");
});
