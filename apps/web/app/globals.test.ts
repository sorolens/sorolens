/**
 * Design-token guard (issue #609).
 *
 * Fails when a component references a `--color-*` token that globals.css does
 * not define in both themes, when a raw Tailwind palette color creeps back
 * into the dashboard, or when a themed text/surface pair drops below WCAG AA.
 */
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, resolve } from "node:path";
import { describe, expect, it } from "vitest";

const WEB_ROOT = resolve(__dirname, "..");
const GLOBALS_CSS = readFileSync(join(__dirname, "globals.css"), "utf8");

function themeBlock(selector: string): string {
  const start = GLOBALS_CSS.indexOf(selector);
  expect(start, `${selector} is missing from globals.css`).toBeGreaterThan(-1);
  const open = GLOBALS_CSS.indexOf("{", start);
  const close = GLOBALS_CSS.indexOf("}", open);
  return GLOBALS_CSS.slice(open + 1, close);
}

const lightBlock = themeBlock(':root[data-theme="light"],\n:root');
const darkBlock = themeBlock(':root[data-theme="dark"]');

function tokens(block: string): Map<string, string> {
  const found = new Map<string, string>();
  for (const match of block.matchAll(/(--[a-z0-9-]+):\s*([^;]+);/gi)) {
    found.set(match[1], match[2].trim());
  }
  return found;
}

const light = tokens(lightBlock);
const dark = tokens(darkBlock);

function sourceFiles(): string[] {
  const files: string[] = [];
  const walk = (dir: string) => {
    for (const entry of readdirSync(dir)) {
      if (entry === "node_modules" || entry === ".next" || entry === "tests") {
        continue;
      }
      const path = join(dir, entry);
      const stat = statSync(path);
      if (stat.isDirectory()) {
        walk(path);
      } else if (/\.(ts|tsx|css)$/.test(entry)) {
        files.push(path);
      }
    }
  };
  walk(WEB_ROOT);
  return files;
}

const RAW_PALETTE =
  /\b(?:bg|text|border|ring|from|to|via|fill|stroke|hover:bg|hover:text|hover:border|dark:bg|dark:text|dark:border)-(?:slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-[0-9]{2,3}\b/;

describe("design tokens", () => {
  it("defines every referenced token in both themes", () => {
    const missing: string[] = [];
    const referenced = new Set<string>();

    for (const file of sourceFiles()) {
      if (file.endsWith("globals.test.ts") || file.endsWith("globals.css")) {
        continue;
      }
      const source = readFileSync(file, "utf8");
      for (const match of source.matchAll(/var\((--[a-z0-9-]+)/gi)) {
        referenced.add(match[1]);
      }
    }

    for (const name of referenced) {
      if (!light.has(name)) missing.push(`${name} (light)`);
      if (!dark.has(name)) missing.push(`${name} (dark)`);
    }

    expect(missing).toEqual([]);
  });

  it("has no raw Tailwind palette colors left in dashboard source", () => {
    const offenders = sourceFiles()
      .filter((file) => !file.endsWith("globals.test.ts"))
      .filter((file) => RAW_PALETTE.test(readFileSync(file, "utf8")))
      .map((file) => file.replace(`${WEB_ROOT}/`, ""));

    expect(offenders).toEqual([]);
  });

  it("keeps themed foreground/background pairs at WCAG AA", () => {
    const failures: string[] = [];

    const pairs: [string, string, string][] = [
      ["text-primary", "bg-page", "text-primary on page"],
      ["text-primary", "bg-card", "text-primary on card"],
      ["text-secondary", "bg-page", "text-secondary on page"],
      ["text-secondary", "bg-card", "text-secondary on card"],
      ["text-muted", "bg-card", "text-muted on card"],
      ["accent", "bg-page", "accent on page"],
      ["accent-text", "accent-bg", "accent-text on accent-bg"],
      ["accent-foreground", "accent", "accent-foreground on accent"],
      ["danger-text", "danger-bg", "danger-text on danger-bg"],
      ["warning-text", "warning-bg", "warning-text on warning-bg"],
      ["safe-text", "safe-bg", "safe-text on safe-bg"],
      ["info-text", "info-bg", "info-text on info-bg"],
      ["danger", "bg-card", "danger text on card"],
      ["warning", "bg-card", "warning text on card"],
      ["safe", "bg-card", "safe text on card"],
      ["info", "bg-card", "info text on card"],
      ["danger-foreground", "danger", "button text on danger"],
      ["warning-foreground", "warning", "button text on warning"],
      ["safe-foreground", "safe", "button text on safe"],
      ["info-foreground", "info", "button text on info"],
      ["chart-foreground", "chart-1", "chart text on series 1"],
      ["chart-foreground", "chart-2", "chart text on series 2"],
      ["chart-foreground", "chart-3", "chart text on series 3"],
      ["chart-foreground", "chart-4", "chart text on series 4"],
      ["chart-foreground", "chart-5", "chart text on series 5"],
      ["chart-foreground", "chart-6", "chart text on series 6"],
      ["chart-foreground", "chart-7", "chart text on series 7"],
    ];

    for (const [themeName, palette] of [
      ["light", light],
      ["dark", dark],
    ] as const) {
      for (const [foreground, background, label] of pairs) {
        const fg = palette.get(`--color-${foreground}`);
        const bg = palette.get(`--color-${background}`);
        if (!fg || !bg) {
          failures.push(`${themeName}: ${label} has an undefined token`);
          continue;
        }
        const ratio = contrastRatio(fg, bg);
        if (ratio < 4.5) {
          failures.push(
            `${themeName}: ${label} is ${ratio.toFixed(2)}:1 (needs 4.5:1)`
          );
        }
      }
    }

    expect(failures).toEqual([]);
  });
});

function luminance(hex: string): number {
  const value = hex.replace("#", "");
  const channels = [0, 2, 4].map((offset) => {
    const channel = parseInt(value.slice(offset, offset + 2), 16) / 255;
    return channel <= 0.03928
      ? channel / 12.92
      : ((channel + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * channels[0] + 0.7152 * channels[1] + 0.0722 * channels[2];
}

function contrastRatio(foreground: string, background: string): number {
  const a = luminance(foreground);
  const b = luminance(background);
  return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
}
