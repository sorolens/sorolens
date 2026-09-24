import { test, expect } from "@playwright/test";
import { blockApi, CONTRACT_ID, expectHeading, mockApi } from "./helpers";

test("/contracts shows the empty state when the API is unavailable", async ({
  page,
}) => {
  await blockApi(page);
  await page.goto("/contracts");

  await expectHeading(page, "Contracts");
  await expect(page.getByText("No contracts tracked yet")).toBeVisible();
});

test("/contracts renders tracked contracts from the mocked API", async ({
  page,
}) => {
  await mockApi(page);
  await page.goto("/contracts");

  await expectHeading(page, "Contracts");
  const table = page.getByRole("table");
  await expect(table.getByText("Escrow DEX")).toBeVisible();
  await expect(table.getByText("testnet", { exact: true })).toBeVisible();
  await expect(table.getByText("active", { exact: true })).toBeVisible();
  // The contract ID is rendered truncated (mono id), so locate by substring.
  await expect(
    table.locator(`text=${CONTRACT_ID.slice(0, 8)}`).first()
  ).toBeVisible();
});

test("/contracts search filters the client-side list", async ({ page }) => {
  await mockApi(page);
  await page.goto("/contracts");

  const search = page.getByLabel("Search contracts");
  const table = page.getByRole("table");

  await search.fill("nope");
  await expect(page.getByText("No contracts match your search")).toBeVisible();

  await search.fill("Escrow");
  await expect(table.getByText("Escrow DEX")).toBeVisible();
});
