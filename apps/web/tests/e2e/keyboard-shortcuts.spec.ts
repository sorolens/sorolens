import { expect, test } from "@playwright/test";
import { expectHeading, mockApi } from "./helpers";

/**
 * End-to-end coverage for the dashboard keyboard shortcuts (issue #831).
 *
 * Shortcuts are sequence based: press `g`, then the second key. They are
 * suppressed while a text field has focus and can be disabled in Settings.
 */

test.describe("Keyboard shortcuts", () => {
  test.beforeEach(async ({ page }) => {
    await mockApi(page);
  });

  test("g then c opens Contracts", async ({ page }) => {
    await page.goto("/events");
    await expectHeading(page, "Events");

    await page.keyboard.press("g");
    await page.keyboard.press("c");

    await expect(page).toHaveURL(/\/contracts$/);
    await expectHeading(page, "Contracts");
  });

  test("g then e opens Events", async ({ page }) => {
    await page.goto("/contracts");
    await expectHeading(page, "Contracts");

    await page.keyboard.press("g");
    await page.keyboard.press("e");

    await expect(page).toHaveURL(/\/events$/);
    await expectHeading(page, "Events");
  });

  test("g then i opens Invocations", async ({ page }) => {
    await page.goto("/contracts");
    await expectHeading(page, "Contracts");

    await page.keyboard.press("g");
    await page.keyboard.press("i");

    await expect(page).toHaveURL(/\/invocations$/);
    await expectHeading(page, "Invocations");
  });

  test("? opens the help dialog and Escape closes it", async ({ page }) => {
    await page.goto("/contracts");
    await expectHeading(page, "Contracts");

    await page.keyboard.press("Shift+/");
    const dialog = page.getByRole("dialog", { name: "Keyboard shortcuts" });
    await expect(dialog).toBeVisible();
    await expect(dialog).toContainText("Go to Contracts");
    await expect(dialog).toContainText("Search contracts");

    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
  });

  test("/ opens the command palette and shortcuts stay off while typing", async ({
    page,
  }) => {
    await page.goto("/contracts");
    await expectHeading(page, "Contracts");

    await page.keyboard.press("/");
    const search = page.getByPlaceholder("Search contracts by ID or label...");
    await expect(search).toBeFocused();

    // The palette input has focus, so "g" then "e" must not navigate.
    await page.keyboard.press("g");
    await page.keyboard.press("e");

    await expect(page).toHaveURL(/\/contracts$/);
  });

  test("Settings can disable shortcuts", async ({ page }) => {
    await page.goto("/settings");
    await expectHeading(page, "Settings");

    const card = page.getByTestId("settings-shortcuts");
    await expect(card).toBeVisible();

    const toggle = card.getByLabel("Enable keyboard shortcuts");
    await expect(toggle).toBeChecked();
    await toggle.uncheck();

    await card.getByRole("button", { name: "Save" }).click();
    await expect(page.getByTestId("settings-shortcuts-status")).toHaveText(
      "Saved"
    );
    await expect(card).toContainText("Shortcuts are currently disabled.");

    // A reload proves the preference persisted, then the shortcut is inert.
    await page.goto("/contracts");
    await expectHeading(page, "Contracts");
    await page.keyboard.press("g");
    await page.keyboard.press("e");

    await expect(page).toHaveURL(/\/contracts$/);
  });
});
