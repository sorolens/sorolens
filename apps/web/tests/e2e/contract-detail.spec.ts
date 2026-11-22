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

  const tabs = page.getByRole("tablist", { name: "Contract sections" });
  await tabs.getByRole("tab", { name: "Events" }).click();
  await expect(page.getByRole("heading", { name: "Events" })).toBeVisible();
  await expect(
    page.getByText("transfer", { exact: false }).first()
  ).toBeVisible();

  await tabs.getByRole("tab", { name: "Storage" }).click();
  await expect(
    page.getByRole("heading", { name: "Storage TTL" })
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Snapshot / replay" })
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
  await page.goto(`/contracts/${CONTRACT_ID}#events`);

  await expect(page.getByRole("heading", { name: "Call trace" })).toBeVisible();

  // The newest event's transaction hash is preloaded, so the flame graph
  // renders without the user typing one.
  const flame = page.getByTestId("trace-flamegraph");
  await expect(flame).toBeVisible();
  await expect(flame.getByText("swap")).toBeVisible();
  await expect(flame.getByText("transfer")).toBeVisible();
  await expect(page.getByText("1 cross-contract call")).toBeVisible();
});

test("contract detail switches to Invocations tab and renders invocations", async ({
  page,
}) => {
  await mockApi(page);
  await page.goto(`/contracts/${CONTRACT_ID}`);

  const tabs = page.getByRole("tablist", { name: "Contract sections" });
  await tabs.getByRole("tab", { name: "Invocations" }).click();
  await expect(
    page.getByRole("combobox", { name: "Filter by status" })
  ).toBeVisible();
  await expect(
    page.getByPlaceholder("Filter by function name...")
  ).toBeVisible();
});

test("contract detail plots health score history and an upgrade marker", async ({
  page,
}) => {
  await mockApi(page);

  await page.route(
    (url) =>
      url.pathname === `/api/v1/contracts/${CONTRACT_ID}/health-score` &&
      url.searchParams.get("history_limit") === "168",
    (route) =>
      route.fulfill({
        json: {
          contract_id: CONTRACT_ID,
          score: 90,
          components: {
            uptime: 95,
            error_rate: 90,
            performance: 85,
            storage_ttl: 85,
          },
          computed_at: "2026-07-03T09:00:00Z",
          history: [
            { computed_at: "2026-07-03T08:00:00Z", score: 80 },
            { computed_at: "2026-07-03T09:00:00Z", score: 90 },
          ],
        },
      })
  );

  await page.route(
    (url) => url.pathname === `/api/v1/contracts/${CONTRACT_ID}/upgrades`,
    (route) =>
      route.fulfill({
        json: {
          upgrades: [{ ledger: 120400, at: "2026-07-03T08:30:00Z" }],
        },
      })
  );

  await page.goto(`/contracts/${CONTRACT_ID}`);

  const chart = page.getByRole("region", { name: "Health score history" });
  await expect(chart.locator(".recharts-line-curve")).toBeVisible();
  await expect(chart.locator(".recharts-reference-line line")).toHaveCount(1);
  await expect(
    chart.getByText("Dashed vertical lines mark contract upgrades.")
  ).toBeVisible();
});
