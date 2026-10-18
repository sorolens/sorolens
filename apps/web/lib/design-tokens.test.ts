/**
 * Design-token audit (issue #609).
 *
 * Three guarantees, enforced by tests so they stay true:
 *
 * 1. No undefined CSS variables — every `var(--color-…)` / `var(--chart-…)`
 *    used in source is defined in BOTH themes in app/globals.css.
 * 2. WCAG AA contrast (4.5:1) for every foreground/background pair the app
 *    composes, in BOTH themes.
 * 3. No raw Tailwind palette colors (`text-red-400` etc.) in components —
 *    they are not designed for light mode. Use semantic tokens instead.
 */
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

// vitest runs with cwd = apps/web.
const WEB_ROOT = process.cwd();
const CSS_PATH = join(WEB_ROOT, "app", "globals.css");
const SCAN_ROOTS = [
  join(WEB_ROOT, "app"),
  join(WEB_ROOT, "components"),
  join(WEB_ROOT, "lib"),
  join(WEB_ROOT, "..", "..", "packages", "ui", "src"),
];

// ---------------------------------------------------------------------------
// CSS parsing
// ---------------------------------------------------------------------------

function themeTokens(): { light: Record<string, string>; dark: Record<string, string> } {
  const css = readFileSync(CSS_PATH, "utf8");
  const parseBlock = (selector: RegExp): Record<string, string> => {
    const m = css.match(selector);
    if (!m) throw new Error(`globals.css: selector ${selector} not found`);
    const block = m[1];
    const out: Record<string, string> = {};
    for (const line of block.split("\n")) {
      const decl = line.match(/^\s*(--[a-z0-9-]+)\s*:\s*([^;]+);/);
      if (decl) out[decl[1]] = decl[2].trim();
    }
    return out;
  };
  return {
    light: parseBlock(/:root\[data-theme="light"\]\s*\{([^}]*)\}/),
    dark: parseBlock(/:root\[data-theme="dark"\]\s*\{([^}]*)\}/),
  };
}

function sourceFiles(): string[] {
  const files: string[] = [];
  const walk = (dir: string) => {
    for (const name of readdirSync(dir)) {
      const p = join(dir, name);
      if (statSync(p).isDirectory()) walk(p);
      else if (/\.(tsx?)$/.test(name) && !/\.test\./.test(name))
        files.push(p);
    }
  };
  SCAN_ROOTS.forEach(walk);
  return files;
}

// ---------------------------------------------------------------------------
// WCAG relative luminance / contrast
// ---------------------------------------------------------------------------

function channel(v: number): number {
  const c = v / 255;
  return c <= 0.04045 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
}

function luminance(hex: string): number {
  const m = hex.match(/^#([0-9a-f]{6})$/i);
  if (!m) throw new Error(`not a 6-digit hex color: ${hex}`);
  const n = parseInt(m[1], 16);
  return (
    0.2126 * channel((n >> 16) & 255) +
    0.7152 * channel((n >> 8) & 255) +
    0.0722 * channel(n & 255)
  );
}

function contrast(fg: string, bg: string): number {
  const a = luminance(fg);
  const b = luminance(bg);
  return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
}

const { light, dark } = themeTokens();
const THEMES = { light, dark } as const;

// ---------------------------------------------------------------------------
// 1. Both themes define the same token keys
// ---------------------------------------------------------------------------

describe("design tokens (#609)", () => {
  it("define the same token keys in both themes", () => {
    expect(Object.keys(light).sort()).toEqual(Object.keys(dark).sort());
    expect(Object.keys(light).length).toBeGreaterThan(20);
  });

  it("have no undefined var(--…) references in source", () => {
    const used = new Set<string>();
    for (const file of sourceFiles()) {
      const src = readFileSync(file, "utf8");
      for (const m of src.matchAll(/var\((--[a-z0-9-]+)[,)]/g)) {
        used.add(m[1]);
      }
    }
    const missing = [...used].filter(
      (token) => !(token in light) || !(token in dark)
    );
    expect(missing).toEqual([]);
    expect(used.size).toBeGreaterThan(5);
  });

  it("use no raw Tailwind palette colors in components", () => {
    const raw =
      /(bg|text|border|ring|fill|stroke)-(slate|zinc|gray|neutral|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-[0-9]/;
    const offenders: string[] = [];
    for (const file of sourceFiles()) {
      const src = readFileSync(file, "utf8");
      if (raw.test(src)) offenders.push(file);
    }
    expect(offenders).toEqual([]);
  });
});

// ---------------------------------------------------------------------------
// 2. WCAG AA contrast for composed pairs, in both themes
// ---------------------------------------------------------------------------

// [foreground, background] pairs actually composed in the UI.
const PAIRS: Array<[fg: string, bg: string, where: string]> = [
  ["--color-text-primary", "--color-bg-page", "body text"],
  ["--color-text-primary", "--color-bg-card", "card text"],
  ["--color-text-secondary", "--color-bg-page", "secondary text"],
  ["--color-text-secondary", "--color-bg-card", "secondary text on card"],
  ["--color-text-muted", "--color-bg-card", "muted text on card"],
  ["--color-accent", "--color-bg-card", "links on card"],
  ["--color-info", "--color-info-bg", "info chip"],
  ["--color-warning", "--color-warning-bg", "warning chip"],
  ["--color-danger", "--color-danger-bg", "danger chip"],
  ["--color-safe", "--color-safe-bg", "safe chip"],
  ["--color-accent", "--color-accent-bg", "accent chip"],
  ["--color-bg-card", "--color-danger", "solid danger button label"],
  ["--color-bg-page", "--color-warning", "solid warning button label"],
];

describe("WCAG AA contrast in both themes (#609)", () => {
  for (const [themeName, tokens] of Object.entries(THEMES)) {
    for (const [fg, bg, where] of PAIRS) {
      it(`${themeName}: ${where} (${fg} on ${bg}) >= 4.5:1`, () => {
        const ratio = contrast(tokens[fg], tokens[bg]);
        expect(ratio).toBeGreaterThanOrEqual(4.5);
      });
    }

    it(`${themeName}: text on chart fills is readable`, () => {
      for (let i = 1; i <= 6; i++) {
        const ratio = contrast(tokens["--color-on-chart"], tokens[`--chart-${i}`]);
        expect(ratio).toBeGreaterThanOrEqual(4.5);
      }
    });
  }
});
