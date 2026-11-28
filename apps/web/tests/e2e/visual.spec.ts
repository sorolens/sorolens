import { readdirSync, readFileSync, statSync } from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";
import { test, expect, type Locator, type Page } from "@playwright/test";
import { CONTRACT_ID, mockApi } from "./helpers";

const THEMES = ["light", "dark"] as const;
type Theme = (typeof THEMES)[number];

/** Upper bound for one committed baseline, to keep the repository small. */
const MAX_BASELINE_BYTES = 150 * 1024;

// A font shipped inside the `next` package, so screenshots do not depend on
// the fonts installed on the machine running the tests.
const require = createRequire(__filename);
const FONT_BASE64 = readFileSync(
  require.resolve("next/dist/compiled/@vercel/og/noto-sans-v27-latin-regular.ttf")
).toString("base64");

const STABLE_CSS = `
  @font-face {
    font-family: "Visual Test";
    src: url(data:font/ttf;base64,${FONT_BASE64}) format("truetype");
  }
  *, *::before, *::after {
    font-family: "Visual Test" !important;
    animation: none !important;
    transition: none !important;
    caret-color: transparent !important;
  }
`;

/** Regions whose content varies between runs; drawn over in every screenshot. */
const VISUAL_MASKS = [
  "time",
  "[data-testid='last-updated']",
  ".recharts-responsive-container",
];

function masks(page: Page): Locator[] {
  return VISUAL_MASKS.map((selector) => page.locator(selector));
}

async function openPage(page: Page, theme: Theme, url: string): Promise<void> {
  await page.clock.setFixedTime(new Date("2026-07-03T12:00:00Z"));
  await page.emulateMedia({ colorScheme: theme, reducedMotion: "reduce" });
  await page.addInitScript((value) => {
    window.localStorage.setItem("theme", value);
  }, theme);
  await mockApi(page);
  await page.goto(url);
  await page.addStyleTag({ content: STABLE_CSS });
  await page.evaluate(() => document.fonts.ready);
  await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
  await page.waitForLoadState("networkidle");
}

const PAGES: { name: string; url: string; ready: (page: Page) => Locator }[] = [
  {
    name: "landing",
    url: "/",
    ready: (page) => page.getByRole("heading", { level: 1 }).first(),
  },
  {
    name: "contracts",
    url: "/contracts",
    ready: (page) => page.getByText("Escrow DEX").first(),
  },
  {
    name: "contract-detail",
    url: `/contracts/${CONTRACT_ID}`,
    ready: (page) => page.getByText("5,041"),
  },
  {
    name: "events",
    url: "/events",
    ready: (page) => page.getByRole("heading", { level: 1 }).first(),
  },
  {
    name: "invocations",
    url: "/invocations",
    ready: (page) => page.getByRole("heading", { level: 1 }).first(),
  },
  {
    name: "watchdog",
    url: "/watchdog",
    ready: (page) => page.getByRole("heading", { level: 1 }).first(),
  },
  {
    name: "settings",
    url: "/settings",
    ready: (page) => page.getByRole("heading", { level: 1 }).first(),
  },
];

const COMPONENTS: {
  name: string;
  url: string;
  target: (page: Page) => Locator;
}[] = [
  {
    name: "contract-tabs",
    url: `/contracts/${CONTRACT_ID}`,
    target: (page) => page.getByRole("tablist", { name: "Contract sections" }),
  },
  {
    name: "contract-stat-cards",
    url: `/contracts/${CONTRACT_ID}`,
    target: (page) => page.locator("#tabpanel-overview > section").first(),
  },
];

test.describe("visual regression", () => {
  test.use({
    viewport: { width: 1280, height: 720 },
    deviceScaleFactor: 1,
    timezoneId: "UTC",
    locale: "en-US",
    // The stabilising stylesheet is injected by the test, not the app.
    bypassCSP: true,
  });

  for (const theme of THEMES) {
    for (const { name, url, ready } of PAGES) {
      test(`${name} page (${theme})`, async ({ page }) => {
        test.skip(
          name === "contracts" && theme === "dark",
          "baseline stale: new Labels nav link; regenerate contracts-dark-chromium-linux.png with the docker command in apps/web/README.md"
        );
        await openPage(page, theme, url);
        await expect(ready(page)).toBeVisible();
        await expect(page).toHaveScreenshot(`${name}-${theme}.png`, {
          animations: "disabled",
          mask: masks(page),
          maxDiffPixelRatio: 0.002,
        });
      });
    }

    for (const { name, url, target } of COMPONENTS) {
      test(`${name} component (${theme})`, async ({ page }) => {
        await openPage(page, theme, url);
        await expect(target(page)).toBeVisible();
        await expect(target(page)).toHaveScreenshot(`${name}-${theme}.png`, {
          animations: "disabled",
          mask: masks(page),
          maxDiffPixelRatio: 0.002,
        });
      });
    }
  }

  test("baselines stay within the size limit", () => {
    const directory = path.join(__dirname, "visual.spec.ts-snapshots");
    const baselines = readdirSync(directory);
    expect(baselines.length).toBeGreaterThanOrEqual(12);
    for (const file of baselines) {
      const { size } = statSync(path.join(directory, file));
      expect(size, `${file} is too large`).toBeLessThanOrEqual(
        MAX_BASELINE_BYTES
      );
    }
  });
});
