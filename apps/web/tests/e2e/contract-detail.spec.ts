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

  // Stat cards read the documented stats payload (issue #586).
  await expect(page.getByText("5,041")).toBeVisible();
  await expect(page.getByText("390", { exact: true })).toBeVisible();
  await expect(page.getByText("Storage Entries")).toBeVisible();
  await expect(page.getByText("128", { exact: true })).toBeVisible();
  await expect(
    page.getByText("Last Synced Ledger", { exact: true })
  ).toBeVisible();
  await expect(page.getByText("120,400", { exact: true })).toBeVisible();
  await expect(page.getByText("1,200 in 7d")).toBeVisible();

  await expect(page.getByRole("heading", { name: "Activity" })).toBeVisible();
  await expect(page.getByText("Event Volume")).toBeVisible();
  await expect(page.getByText("Invocations").first()).toBeVisible();

  // Both charts are derived from paged events/invocations and rendered.
  await expect(page.locator(".recharts-wrapper")).toHaveCount(2);
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

test("contract detail renders the cross-contract call trace", async ({
  page,
}) => {
  await mockApi(page);
  await page.goto(`/contracts/${CONTRACT_ID}`);

  await expect(page.getByRole("heading", { name: "Call trace" })).toBeVisible();

  // The newest event's transaction hash is preloaded, so the flame graph
  // renders without the user typing one.
  const flame = page.getByTestId("trace-flamegraph");
  await expect(flame).toBeVisible();
  await expect(flame.getByText("swap")).toBeVisible();
  await expect(flame.getByText("transfer")).toBeVisible();
  await expect(page.getByText("1 cross-contract call")).toBeVisible();
});
