import { expect, test, type Page } from "@playwright/test";
import { expectHeading, mockApi } from "./helpers";

/**
 * End-to-end coverage for `/settings`.
 *
 * The page persists preferences to `localStorage` rather than the API, so each
 * test saves, reloads, and asserts that the value came back. Every section is
 * driven the way a user would: fill the field, press that card's Save button,
 * then reload.
 *
 * The cards render only after the page has read stored preferences (see the
 * `loaded` flag in page.tsx), so waiting for a card is also the hydration gate
 * — no interaction can be lost to the pre-hydration render.
 */

const SETTINGS_KEY = "sorolens_settings";

function networkSelect(page: Page) {
  // The section is also named "Default network" via aria-labelledby, so scope
  // to the card before asking for the labelled control.
  return page.getByTestId("settings-network").getByLabel("Default network");
}

test.describe("Settings", () => {
  test.beforeEach(async ({ page }) => {
    await mockApi(page);
    await page.goto("/settings");
    await expectHeading(page, "Settings");
    await expect(page.getByTestId("settings-theme")).toBeVisible();
  });

  test("renders every section with its defaults", async ({ page }) => {
    await expect(page.getByTestId("settings-network")).toBeVisible();
    await expect(page.getByTestId("settings-notifications")).toBeVisible();
    await expect(page.getByTestId("settings-api-key")).toBeVisible();

    await expect(networkSelect(page)).toHaveValue("all");
    await expect(page.getByLabel("Email address")).toHaveValue("");
    await expect(page.getByLabel("Critical alerts")).toBeChecked();
    await expect(page.getByLabel("Weekly digest")).not.toBeChecked();
    await expect(page.getByTestId("settings-api-key-value")).toHaveText("—");
  });

  test("saves the theme and keeps it after a reload", async ({ page }) => {
    // Scoped to the radio: the header toggle also has an aria-label with "Dark".
    await page.getByRole("radio", { name: "Dark", exact: true }).check();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");

    await page
      .getByTestId("settings-theme")
      .getByRole("button", { name: "Save" })
      .click();
    await expect(page.getByTestId("settings-theme-status")).toHaveText("Saved");

    await page.reload();
    await expectHeading(page, "Settings");
    await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
    await expect(
      page.getByRole("radio", { name: "Dark", exact: true })
    ).toBeChecked();
  });

  test("saves the default network and applies it to the header selector", async ({
    page,
  }) => {
    await networkSelect(page).selectOption("futurenet");
    await page
      .getByTestId("settings-network")
      .getByRole("button", { name: "Save" })
      .click();
    await expect(page.getByTestId("settings-network-status")).toHaveText(
      "Saved"
    );
    await expect(page.locator("#network-selector")).toHaveValue("futurenet");

    await page.reload();
    await expect(networkSelect(page)).toHaveValue("futurenet");
  });

  test("rejects a malformed notification email without saving", async ({
    page,
  }) => {
    await page.getByLabel("Email address").fill("not-an-email");
    await expect(
      page.getByTestId("settings-notifications-error")
    ).toBeVisible();

    await page
      .getByTestId("settings-notifications")
      .getByRole("button", { name: "Save" })
      .click();
    await expect(page.getByTestId("settings-notifications-status")).toHaveCount(
      0
    );

    await page.reload();
    await expect(page.getByLabel("Email address")).toHaveValue("");
    await expect(page.getByTestId("settings-notifications-error")).toHaveCount(
      0
    );
  });

  test("saves notification preferences", async ({ page }) => {
    await page.getByLabel("Email address").fill("dev@example.com");
    await page.getByLabel("Weekly digest").check();
    await page.getByLabel("Health changes").uncheck();

    await page
      .getByTestId("settings-notifications")
      .getByRole("button", { name: "Save" })
      .click();
    await expect(page.getByTestId("settings-notifications-status")).toHaveText(
      "Saved"
    );

    await page.reload();
    await expectHeading(page, "Settings");
    await expect(page.getByTestId("settings-theme")).toBeVisible();
    await expect(page.getByLabel("Email address")).toHaveValue(
      "dev@example.com"
    );
    await expect(page.getByLabel("Weekly digest")).toBeChecked();
    await expect(page.getByLabel("Health changes")).not.toBeChecked();
  });

  test("generates, saves, and revokes an API key", async ({ page }) => {
    await page
      .getByTestId("settings-api-key")
      .getByRole("button", { name: "Generate key" })
      .click();
    await expect(page.getByTestId("settings-api-key-value")).toContainText(
      "sl_live_"
    );

    await page
      .getByTestId("settings-api-key")
      .getByRole("button", { name: "Save" })
      .click();
    await expect(page.getByTestId("settings-api-key-status")).toHaveText(
      "Saved"
    );

    // Reloading masks the token again, but the key is still stored.
    await page.reload();
    await expect(page.getByTestId("settings-theme")).toBeVisible();
    await expect(page.getByTestId("settings-api-key-value")).toContainText("•");
    await expect(page.getByTestId("settings-api-key-value")).toContainText(
      "sl_live_"
    );

    await page
      .getByTestId("settings-api-key")
      .getByRole("button", { name: "Revoke" })
      .click();
    await page
      .getByTestId("settings-api-key")
      .getByRole("button", { name: "Save" })
      .click();
    await expect(page.getByTestId("settings-api-key-value")).toHaveText("—");

    await page.reload();
    await expect(page.getByTestId("settings-theme")).toBeVisible();
    await expect(page.getByTestId("settings-api-key-value")).toHaveText("—");
    const stored = await page.evaluate(
      (key) => window.localStorage.getItem(key),
      SETTINGS_KEY
    );
    expect(JSON.parse(stored as string).apiKey).toBeNull();
  });

  test("is reachable from the header navigation", async ({ page }) => {
    await page.goto("/contracts");
    const nav = page.locator("header nav");
    await nav.getByRole("link", { name: "Settings" }).click();
    await expect(page).toHaveURL(/\/settings$/);
    await expectHeading(page, "Settings");
    await expect(page.getByTestId("settings-theme")).toBeVisible();
  });
});
