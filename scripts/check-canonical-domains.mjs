#!/usr/bin/env node
/**
 * Canonical domain consistency check (issue #569).
 *
 * Scans tracked repository files to verify that deprecated/stale domains
 * are not used anywhere.
 *
 * Canonical domains:
 *   - Web application: https://sorolens.xyz
 *   - API: https://api.sorolens.xyz
 *   - Project emails: *@sorolens.xyz
 *
 * Banned patterns:
 *   - sorolens.dev
 *   - sorolens-web-iota.vercel.app
 *   - api.sorolens.com
 */

import { readdirSync, readFileSync, statSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const rootDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

const IGNORED_DIRS = new Set([
  ".git",
  "node_modules",
  ".next",
  "dist",
  "build",
  "coverage",
  "test-results",
  "playwright-report",
  "target",
  ".turbo",
  ".cache",
  ".pnpm-store",
]);

const IGNORED_EXTENSIONS = new Set([
  ".png",
  ".jpg",
  ".jpeg",
  ".ico",
  ".svg",
  ".wasm",
  ".tar.gz",
  ".gz",
  ".zip",
]);

const IGNORED_FILES = new Set([
  "check-canonical-domains.mjs",
  "pnpm-lock.yaml",
  "package-lock.json",
  "Cargo.lock",
]);

const BANNED_PATTERNS = [
  /sorolens\.dev/,
  /sorolens-web-iota\.vercel\.app/,
  /api\.sorolens\.com/,
];

function scanDirectory(dirPath, violations) {
  let entries;
  try {
    entries = readdirSync(dirPath, { withFileTypes: true });
  } catch {
    return;
  }

  for (const entry of entries) {
    const fullPath = path.join(dirPath, entry.name);
    const relPath = path.relative(rootDir, fullPath);

    if (entry.isDirectory()) {
      if (IGNORED_DIRS.has(entry.name)) continue;
      scanDirectory(fullPath, violations);
    } else if (entry.isFile()) {
      if (IGNORED_FILES.has(entry.name)) continue;
      const ext = path.extname(entry.name).toLowerCase();
      if (IGNORED_EXTENSIONS.has(ext)) continue;

      try {
        const content = readFileSync(fullPath, "utf8");
        const lines = content.split("\n");
        for (let i = 0; i < lines.length; i++) {
          const line = lines[i];
          for (const pattern of BANNED_PATTERNS) {
            if (pattern.test(line)) {
              violations.push(`${relPath}:${i + 1}:${line.trim()}`);
              break;
            }
          }
        }
      } catch {
        // Binary or unreadable file; skip
      }
    }
  }
}

const violations = [];
scanDirectory(rootDir, violations);

if (violations.length > 0) {
  console.error("❌ Error: Stale non-canonical domain reference(s) found:\n");
  for (const v of violations) {
    console.error(v);
  }
  console.error(
    "\nPlease use canonical domains: https://sorolens.xyz (web) or https://api.sorolens.xyz (API)."
  );
  process.exit(1);
} else {
  console.log("✔ Canonical domain check passed: no stale domain references found.");
  process.exit(0);
}
