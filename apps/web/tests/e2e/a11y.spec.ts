import { expect, test, type Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { expectHeading, mockApi } from "./helpers";

/**
 * Automated accessibility checks (issue #611).
 *
 * Every key page is scanned with axe-core; serious and critical violations
 * fail the test so a regression cannot merge silently. The axe scans run once
 * per run (desktop project); the mobile projects still execute the rest of the
 * suite, and one responsive smoke test below asserts the mobile shell works.
 */

const PAGES: { path: string; heading: string | RegExp }[] = [
  { path: "/", heading: /Real-time monitoring/i },
  { path: "/contracts", heading: "Contracts" },
  { path: "/watchdog", heading: /Watchdog/i },
  { path: "/settings", heading: "Settings" },
  { path: "/integrations", heading: "Integrations" },
  { path: "/playground", heading: "API Playground" },
];

const SERIOUS = new Set(["serious", "critical"]);

async function seriousViolations(page: Page) {
  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa"])
    .analyze();
  return results.violations
    .filter((violation) => SERIOUS.has(violation.impact ?? ""))
    .map((violation) => ({
      id: violation.id,
      impact: violation.impact,
      help: violation.help,
      nodes: violation.nodes.map((node) => node.target.join(" ")),
    }));
}

test.describe("accessibility", () => {
  // Mobile duplicates would re-scan the same DOM; the mobile projects still
  // run their own smoke coverage below.
  test.skip(({ isMobile }) => Boolean(isMobile), "axe runs once per page");

  for (const entry of PAGES) {
    test(`${entry.path} has no serious axe violations`, async ({ page }) => {
      await mockApi(page);
      await page.goto(entry.path);
      await expectHeading(page, entry.heading, { exact: false });

      const violations = await seriousViolations(page);
      expect(
        violations,
        `serious/critical axe violations on ${entry.path}:\n${JSON.stringify(
          violations,
          null,
          2
        )}`
      ).toEqual([]);
    });
  }

  test("contracts table exposes column headers and row links", async ({
    page,
  }) => {
    await mockApi(page);
    await page.goto("/contracts");

    // #610: real links in the row and sortable headers announced as columns.
    await expect(page.getByRole("link", { name: /Escrow DEX/ })).toBeVisible();
    await expect(
      page.getByRole("columnheader", { name: /Contract ID/i })
    ).toHaveAttribute("aria-sort", "none");
  });
});

test.describe("mobile shell", () => {
  test.skip(({ isMobile }) => !isMobile, "mobile projects only");

  test("key pages render without horizontal overflow", async ({ page }) => {
    await mockApi(page);
    await page.goto("/contracts");
    await expectHeading(page, "Contracts");

    const overflow = await page.evaluate(
      () =>
        document.documentElement.scrollWidth -
        document.documentElement.clientWidth
    );
    expect(overflow).toBeLessThanOrEqual(1);
  });

  test("settings is usable on a mobile viewport", async ({ page }) => {
    await mockApi(page);
    await page.goto("/settings");
    await expectHeading(page, "Settings");
    await expect(page.getByTestId("settings-api-key")).toBeVisible();
    await expect(page.getByTestId("settings-identity")).toBeVisible();
  });

  test("the header navigation exposes the integrations page", async ({
    page,
  }) => {
    await mockApi(page);
    await page.goto("/");
    await expect(
      page.locator("header nav").getByRole("link", { name: "Integrations" })
    ).toBeVisible();
  });
});
