import { test, expect } from "@playwright/test";
import { CONTRACT_ID, mockApi } from "./helpers";

/**
 * Contract tracking wizard (issue #140).
 *
 * Acceptance: renders on /contracts/new, every step validates and has
 * Next/Back, successful creation redirects to /contracts/[id].
 */

test.beforeEach(async ({ page }) => {
  await mockApi(page);
});

test("the Track contract button now opens the wizard route", async ({
  page,
}) => {
  await page.goto("/contracts");
  await page
    .getByRole("link", { name: /track contract/i })
    .first()
    .click();

  await expect(page).toHaveURL(/\/contracts\/new$/);
  await expect(
    page.getByRole("heading", { name: "Track a contract" })
  ).toBeVisible();
  // The old single-field modal is gone.
  await expect(page.getByRole("dialog")).toHaveCount(0);
});

test("the full wizard flow creates a contract and redirects to its detail page", async ({
  page,
}) => {
  await page.goto("/contracts/new");

  // Step 1: identify ---------------------------------------------------------
  const next = page.getByTestId("wizard-next");
  await expect(next).toBeDisabled();

  await page.getByTestId("wizard-contract-id").fill(CONTRACT_ID);
  await page.getByTestId("wizard-network").selectOption("testnet");
  await expect(next).toBeEnabled();

  await next.click();

  // Step 2: validate ---------------------------------------------------------
  await expect(page.getByTestId("wizard-valid")).toBeVisible();
  await expect(page.getByTestId("wizard-next")).toBeEnabled();
  await page.getByTestId("wizard-next").click();

  // Step 3: configure --------------------------------------------------------
  await expect(page.getByTestId("wizard-step-configure")).toBeVisible();
  await page.getByTestId("wizard-label").fill("Escrow DEX");
  await page.getByTestId("wizard-create").click();

  await expect(page).toHaveURL(new RegExp(`/contracts/${CONTRACT_ID}$`));
});

test("step 1 rejects a malformed contract id and never advances", async ({
  page,
}) => {
  await page.goto("/contracts/new");

  await page.getByTestId("wizard-contract-id").fill("NOT-A-REAL-ID");
  await expect(page.getByTestId("wizard-contract-id-error")).toBeVisible();
  await expect(page.getByTestId("wizard-next")).toBeDisabled();

  // Still on step 1: the label input belongs to step 3.
  await expect(page.getByTestId("wizard-label")).toHaveCount(0);
});

test("step 2 surfaces an invalid contract from the API", async ({ page }) => {
  await mockApi(page, {
    "contracts/validate": () => ({
      status: 200,
      body: {
        valid: false,
        contract_id: CONTRACT_ID,
        network: "testnet",
        already_tracked: false,
        label: null,
        reason:
          "contract id checksum does not match (check for a mistyped character)",
      },
    }),
  });

  await page.goto("/contracts/new");
  await page.getByTestId("wizard-contract-id").fill(CONTRACT_ID);
  await page.getByTestId("wizard-next").click();

  await expect(page.getByTestId("wizard-invalid")).toContainText("checksum");
  await expect(page.getByTestId("wizard-next")).toBeDisabled();
});

test("step 2 points at the existing contract when it is already tracked", async ({
  page,
}) => {
  await mockApi(page, {
    "contracts/validate": () => ({
      status: 200,
      body: {
        valid: true,
        contract_id: CONTRACT_ID,
        network: "testnet",
        already_tracked: true,
        label: "Escrow DEX",
        reason: null,
      },
    }),
  });

  await page.goto("/contracts/new");
  await page.getByTestId("wizard-contract-id").fill(CONTRACT_ID);
  await page.getByTestId("wizard-next").click();

  const tracked = page.getByTestId("wizard-already-tracked");
  await expect(tracked).toBeVisible();
  await expect(tracked).toContainText("Escrow DEX");
  await expect(tracked.getByRole("link")).toHaveAttribute(
    "href",
    `/contracts/${CONTRACT_ID}`
  );
  // Creating a duplicate is blocked.
  await expect(page.getByTestId("wizard-next")).toBeDisabled();
});

test("Back returns to step 1 with the entered contract id preserved", async ({
  page,
}) => {
  await page.goto("/contracts/new");
  await page.getByTestId("wizard-contract-id").fill(CONTRACT_ID);
  await page.getByTestId("wizard-next").click();
  await expect(page.getByTestId("wizard-valid")).toBeVisible();

  await page.getByTestId("wizard-back").click();
  await expect(page.getByTestId("wizard-contract-id")).toHaveValue(CONTRACT_ID);
  await expect(page.getByTestId("wizard-next")).toBeEnabled();
});

test("Back is disabled on the first step", async ({ page }) => {
  await page.goto("/contracts/new");
  await expect(page.getByTestId("wizard-back")).toBeDisabled();
});

test("a failed creation keeps the user on step 3 and shows the API error", async ({
  page,
}) => {
  await mockApi(page, {
    contracts: (route) =>
      route.request().method() === "POST"
        ? { status: 409, body: { error: "contract already being tracked" } }
        : {
            status: 200,
            body: { contracts: [], cursor: null, has_more: false },
          },
  });

  await page.goto("/contracts/new");
  await page.getByTestId("wizard-contract-id").fill(CONTRACT_ID);
  await page.getByTestId("wizard-next").click();
  await page.getByTestId("wizard-next").click();
  await page.getByTestId("wizard-create").click();

  await expect(page.getByTestId("wizard-create-error")).toContainText(
    "already being tracked"
  );
  await expect(page).toHaveURL(/\/contracts\/new$/);
});
