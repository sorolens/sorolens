#!/usr/bin/env node
/**
 * Unused-export guard for apps/web (issue #760).
 *
 * Runs knip against apps/web to detect dead components, hooks, utils,
 * exports, and types. Any finding not explicitly documented in
 * apps/web/unused-exports.allowlist.json will fail CI.
 *
 * Usage:
 *   node scripts/check-unused-exports.mjs
 */

import { readFileSync, existsSync } from "node:fs";
import { execSync } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";

const webRoot = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  ".."
);
const allowlistPath = path.join(webRoot, "unused-exports.allowlist.json");

if (!existsSync(allowlistPath)) {
  console.error(
    "Missing allow-list file: apps/web/unused-exports.allowlist.json"
  );
  process.exit(1);
}

const allowlist = JSON.parse(readFileSync(allowlistPath, "utf8"));
const allowedFiles = new Set((allowlist.files ?? []).map((f) => f.path));
const allowedExports = new Set(
  (allowlist.exports ?? []).map((e) => `${e.file}::${e.name}`)
);
const allowedTypes = new Set(
  (allowlist.types ?? []).map((t) => `${t.file}::${t.name}`)
);

console.log("Checking for dead exports and unused files in apps/web...");

let rawOutput = "";
try {
  // Knip exits with code 1 if any issues are found
  rawOutput = execSync(
    "pnpm exec knip --include files,exports,types --reporter json",
    {
      cwd: webRoot,
      encoding: "utf8",
      stdio: ["ignore", "pipe", "pipe"],
    }
  );
} catch (err) {
  // Knip writes JSON output to stdout even when exiting with non-zero code
  rawOutput = err.stdout ? err.stdout.toString() : "";
  if (!rawOutput && err.stderr) {
    console.error("Knip execution failed:", err.stderr.toString());
    process.exit(1);
  }
}

// Find JSON substring if knip logged preamble
let jsonStart = rawOutput.indexOf("{");
if (jsonStart === -1) {
  console.log(
    "✓ No issues reported by knip. All components and exports are used."
  );
  process.exit(0);
}

let report;
try {
  report = JSON.parse(rawOutput.slice(jsonStart));
} catch (e) {
  console.error("Failed to parse knip JSON output:", e);
  console.error(rawOutput);
  process.exit(1);
}

const violations = [];

for (const issue of report.issues ?? []) {
  const filePath = issue.file;

  // 1. Check unused files
  if (issue.files && issue.files.length > 0) {
    if (!allowedFiles.has(filePath)) {
      violations.push({
        type: "Unused File",
        location: filePath,
        name: path.basename(filePath),
      });
    }
  }

  // 2. Check unused exports
  for (const exp of issue.exports ?? []) {
    const key = `${filePath}::${exp.name}`;
    if (!allowedExports.has(key)) {
      violations.push({
        type: "Unused Export",
        location: `${filePath}:${exp.line}:${exp.col}`,
        name: exp.name,
      });
    }
  }

  // 3. Check unused types
  for (const type of issue.types ?? []) {
    const key = `${filePath}::${type.name}`;
    if (!allowedTypes.has(key)) {
      violations.push({
        type: "Unused Type",
        location: `${filePath}:${type.line}:${type.col}`,
        name: type.name,
      });
    }
  }
}

if (violations.length > 0) {
  console.error(
    `\n❌ ${violations.length} unexpected unused export(s) / file(s) found in apps/web:\n`
  );
  for (const v of violations) {
    console.error(`  - [${v.type}] ${v.location} -> "${v.name}"`);
  }
  console.error(
    "\nFix: Delete the unused file/export (and any tests covering only it)."
  );
  console.error(
    "If this export is intentional (e.g. public API client, reusable primitive), document it"
  );
  console.error(
    "with a clear reason in apps/web/unused-exports.allowlist.json.\n"
  );
  process.exit(1);
}

console.log(
  "✓ Unused-export check passed: no dead components, hooks, or utils found."
);
