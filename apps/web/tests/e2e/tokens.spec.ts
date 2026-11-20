import { test, expect } from "@playwright/test";
import { CONTRACT_ID, TRACE_TX_HASH, mockApi } from "./helpers";

const SENDER = "GD2E2SJSD2C5SQGMH2YPIXI6F7NS2S6MZ2G4T3VDWXQH2FQT4M6URPLT";
const RECEIVER = "GCQH32AZF6PXY2R4T3M5Q7XFYVK2TLQD2TELC7K4BXL5Q6T7P9U2KNQT";

const tokenHandlers = {
  "/transfers": () => ({
    status: 200,
    body: {
      transfers: [
        {
          id: "transfer_1",
          kind: "transfer",
          from: SENDER,
          to: RECEIVER,
          amount: "12345678900000",
          tx_hash: TRACE_TX_HASH,
          ledger: 120_400,
          ledger_closed_at: "2026-07-03T08:00:00Z",
        },
      ],
      next_cursor: "",
      decimals: 7,
    },
  }),
  "/holders": () => ({
    status: 200,
    body: {
      holders: [
        { address: RECEIVER, balance: "75000000" },
        { address: SENDER, balance: "25000000" },
      ],
      next_cursor: "",
      decimals: 7,
      total_supply: "100000000",
    },
  }),
};

test("token contracts get a Tokens tab with transfers and holders", async ({
  page,
}) => {
  await mockApi(page, tokenHandlers);
  await page.goto(`/contracts/${CONTRACT_ID}`);

  const tabs = page.getByRole("tablist", { name: "Contract sections" });
  await tabs.getByRole("tab", { name: "Tokens" }).click();

  await expect(page.getByRole("heading", { name: "Transfers" })).toBeVisible();
  await expect(page.getByText("1,234,567.89")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Holders" })).toBeVisible();
  await expect(page.getByText("75.00%")).toBeVisible();
  await expect(page.getByTestId("holder-distribution")).toBeVisible();
});

test("the transfers export downloads a CSV", async ({ page }) => {
  await mockApi(page, tokenHandlers);
  await page.goto(`/contracts/${CONTRACT_ID}#tokens`);

  const transfers = page.getByRole("region", { name: "Transfers" });
  const download = page.waitForEvent("download");
  await transfers.getByRole("button", { name: "Export CSV" }).click();

  expect((await download).suggestedFilename()).toBe(
    `${CONTRACT_ID}-transfers.csv`
  );
});

test("contracts without token data do not show the tab", async ({ page }) => {
  await mockApi(page);
  await page.goto(`/contracts/${CONTRACT_ID}#tokens`);

  const tabs = page.getByRole("tablist", { name: "Contract sections" });
  await expect(tabs.getByRole("tab", { name: "Overview" })).toHaveAttribute(
    "aria-selected",
    "true"
  );
  await expect(tabs.getByRole("tab", { name: "Tokens" })).toHaveCount(0);
});
