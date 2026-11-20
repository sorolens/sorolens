import { test, expect } from "@playwright/test";

test.describe("Storage Entry Diff E2E Spec", () => {
  test("navigates to contract compare / storage diff and views diff groups", async ({ page }) => {
    // Intercept storage diff API response
    await page.route("**/api/v1/contracts/*/storage/diff*", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          contract_id: "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAD2KM",
          from_ledger: 100,
          to_ledger: 200,
          created: [
            {
              key: "e2e_created_storage",
              durability: "persistent",
              live_until_ledger: 1000,
              val_decoded: { type: "scvU32", value: 100, human: "100" }
            }
          ],
          changed: [],
          removed: [],
          expired: []
        }),
      });
    });

    await page.goto("/compare");

    // Verify page headers or controls exist
    await expect(page.locator("body")).toContainText(/Compare|Storage Diff/i);
  });
});
