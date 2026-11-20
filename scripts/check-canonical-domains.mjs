#!/usr/bin/env node
/**
 * Canonical domain consistency check (issue #569).
 *
 * Verifies that deprecated/stale domains are not used anywhere in the repository.
 * Canonical domains:
 *   - Web application: https://sorolens.xyz
 *   - API: https://api.sorolens.xyz
 *   - Project emails: *@sorolens.xyz
 *
 * Banned domain patterns:
 *   - sorolens.dev
 *   - sorolens-web-iota.vercel.app
 *   - api.sorolens.com / sorolens.com (as API/web domain)
 *
 * Usage:
 *   node scripts/check-canonical-domains.mjs
 */

import { execSync } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";

const rootDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

// Regex matching deprecated domains:
// - sorolens.dev
// - sorolens-web-iota.vercel.app
// - api.sorolens.com (or sorolens.com as a host/email)
const BANNED_PATTERN = "(sorolens\\.dev|sorolens-web-iota\\.vercel\\.app|api\\.sorolens\\.com)";

const EXCLUDE_PATHS = [
  ":!scripts/check-canonical-domains.mjs",
  ":!node_modules",
  ":!.git",
];

try {
  const cmd = `git grep -nE "${BANNED_PATTERN}" -- ${EXCLUDE_PATHS.map((p) => `"${p}"`).join(" ")}`;
  const stdout = execSync(cmd, {
    cwd: rootDir,
    encoding: "utf8",
    stdio: ["pipe", "pipe", "pipe"],
  }).trim();

  if (stdout.length > 0) {
    console.error("❌ Error: Stale non-canonical domain reference(s) found:\n");
    console.error(stdout);
    console.error(
      "\nPlease use canonical domains: https://sorolens.xyz (web) or https://api.sorolens.xyz (API)."
    );
    process.exit(1);
  }
} catch (err) {
  // git grep exits with 1 when no matches are found, which is what we want!
  if (err.status === 1) {
    console.log("✔ Canonical domain check passed: no stale domain references found.");
    process.exit(0);
  }

  // Any other error (e.g. git error) should be reported:
  console.error("Failed to run git grep domain check:", err.message);
  process.exit(1);
}
