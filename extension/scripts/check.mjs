// Local verification for the extension: syntax-check every JS module and
// validate both manifests (MV3 shape + every referenced file exists).
//
// Usage: node scripts/check.mjs
//
// This is deliberately dependency-free so it runs the same way on a laptop and
// in CI without an install step.

import { execFileSync } from "node:child_process";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const SKIP_DIRS = new Set(["node_modules", "dist", ".git"]);
const MANIFESTS = ["manifest.json", "manifest.firefox.json"];

const failures = [];
let checkedFiles = 0;

function walk(dir) {
  const entries = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    if (SKIP_DIRS.has(entry.name)) continue;
    const full = join(dir, entry.name);
    if (entry.isDirectory()) entries.push(...walk(full));
    else entries.push(full);
  }
  return entries;
}

const allFiles = walk(ROOT);

// ---- 1. syntax-check every module ------------------------------------------

for (const file of allFiles.filter((f) => /\.(js|mjs)$/.test(f))) {
  checkedFiles += 1;
  try {
    execFileSync(process.execPath, ["--check", file], { stdio: "pipe" });
  } catch (error) {
    failures.push(
      `${relative(ROOT, file)}: node --check failed\n${error.stderr?.toString() || error.message}`,
    );
  }
}

// ---- 2. manifest validation ------------------------------------------------

/** Resolves a `resources` glob entry to the number of files it matches. */
function matchesResource(pattern) {
  const star = pattern.indexOf("*");
  if (star === -1) {
    return existsSync(join(ROOT, pattern)) ? 1 : 0;
  }
  const prefix = pattern.slice(0, star);
  const suffix = pattern.slice(star + 1);
  const slash = prefix.lastIndexOf("/");
  const dir = join(ROOT, slash === -1 ? "" : prefix.slice(0, slash));
  const base = slash === -1 ? "" : prefix.slice(slash + 1);
  if (!existsSync(dir)) return 0;
  return readdirSync(dir).filter(
    (name) => name.startsWith(base) && name.endsWith(suffix),
  ).length;
}

for (const name of MANIFESTS) {
  const path = join(ROOT, name);
  if (!existsSync(path)) {
    failures.push(`${name}: missing`);
    continue;
  }
  let manifest;
  try {
    manifest = JSON.parse(readFileSync(path, "utf8"));
  } catch (error) {
    failures.push(`${name}: invalid JSON (${error.message})`);
    continue;
  }

  if (manifest.manifest_version !== 3) {
    failures.push(`${name}: manifest_version must be 3`);
  }
  for (const key of ["name", "version", "description"]) {
    if (!manifest[key]) failures.push(`${name}: missing "${key}"`);
  }

  // Background: Chrome uses a service worker, Firefox an event page.
  const background = manifest.background || {};
  const isFirefoxManifest = name.includes("firefox");
  if (isFirefoxManifest) {
    if (!Array.isArray(background.scripts) || background.scripts.length === 0) {
      failures.push(`${name}: background.scripts is required for Firefox`);
    }
    if (!manifest.browser_specific_settings?.gecko?.id) {
      failures.push(`${name}: browser_specific_settings.gecko.id is required`);
    }
  } else if (!background.service_worker) {
    failures.push(`${name}: background.service_worker is required for Chrome`);
  }

  // Host permissions must cover the API and the three explorer origins.
  const hosts = manifest.host_permissions || [];
  for (const required of [
    "https://api.sorolens.xyz/*",
    "https://stellar.expert/*",
    "https://lab.stellar.org/*",
    "https://stellar.org/*",
  ]) {
    if (!hosts.includes(required)) {
      failures.push(`${name}: host_permissions is missing ${required}`);
    }
  }

  const contentScripts = manifest.content_scripts || [];
  const matches = new Set(contentScripts.flatMap((script) => script.matches || []));
  for (const origin of [
    "https://stellar.expert/*",
    "https://lab.stellar.org/*",
    "https://stellar.org/*",
  ]) {
    if (!matches.has(origin)) {
      failures.push(`${name}: content_scripts is missing a match for ${origin}`);
    }
  }
  if (contentScripts.length === 0) {
    failures.push(`${name}: content_scripts is empty`);
  }

  // Every referenced file must exist, and globs must match something.
  const referenced = [
    ...Object.values(manifest.icons || {}),
    ...Object.values(manifest.action?.default_icon || {}),
    manifest.action?.default_popup,
    manifest.options_ui?.page,
    background.service_worker,
    ...(background.scripts || []),
    ...contentScripts.flatMap((script) => [
      ...(script.js || []),
      ...(script.css || []),
    ]),
  ].filter(Boolean);
  for (const file of referenced) {
    if (!existsSync(join(ROOT, file))) {
      failures.push(`${name}: referenced file does not exist: ${file}`);
    }
  }

  const resources = (manifest.web_accessible_resources || []).flatMap(
    (entry) => entry.resources || [],
  );
  if (resources.length === 0) {
    failures.push(`${name}: web_accessible_resources is empty`);
  }
  for (const pattern of resources) {
    if (matchesResource(pattern) === 0) {
      failures.push(`${name}: web_accessible_resources matches nothing: ${pattern}`);
    }
  }
}

// ---- 3. HTML asset references ----------------------------------------------

const HTML_FILES = ["src/popup/popup.html", "src/options/options.html"];
for (const file of HTML_FILES) {
  const source = readFileSync(join(ROOT, file), "utf8");
  for (const [, ref] of source.matchAll(/(?:src|href)="([^"]+)"/g)) {
    if (/^(https?:|data:|#)/.test(ref)) continue;
    const target = resolve(join(ROOT, dirname(file)), ref);
    if (!existsSync(target)) {
      failures.push(`${file}: references missing asset ${ref}`);
    }
  }
}

// ---- 4. report -------------------------------------------------------------

console.log(`node --check: ${checkedFiles} files`);
console.log(`manifests: ${MANIFESTS.join(", ")}`);
if (failures.length > 0) {
  console.error(`\n${failures.length} problem(s):`);
  for (const failure of failures) console.error(`  - ${failure}`);
  process.exit(1);
}
console.log("all checks passed");
