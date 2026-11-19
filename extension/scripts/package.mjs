// Store-ready packaging: build each browser target and zip it.
//
// Usage:
//   node scripts/package.mjs            # both targets
//   node scripts/package.mjs chrome
//   node scripts/package.mjs firefox
//
// Output: dist/sorolens-<version>-chrome.zip and ...-firefox.zip, each with
// manifest.json at the archive root (what both stores expect). The zips are
// built from dist/<target>/, which build.mjs produces from one src/ tree.

import { execFileSync } from "node:child_process";
import {
  existsSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  rmSync,
  statSync,
} from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const DIST = join(ROOT, "dist");
const TARGETS = ["chrome", "firefox"];

const requested = (process.argv[2] || "all").toLowerCase();
if (requested !== "all" && !TARGETS.includes(requested)) {
  console.error(`Usage: node scripts/package.mjs [all|${TARGETS.join("|")}]`);
  process.exit(1);
}
const targets = requested === "all" ? TARGETS : [requested];

function manifestVersion(target) {
  const file =
    target === "firefox" ? "manifest.firefox.json" : "manifest.json";
  const manifest = JSON.parse(readFileSync(join(ROOT, file), "utf8"));
  return manifest.version;
}

/** Refuses to zip two targets whose versions drifted apart. */
function assertVersionsMatch() {
  const versions = new Map(TARGETS.map((target) => [target, manifestVersion(target)]));
  const unique = new Set(versions.values());
  if (unique.size !== 1) {
    console.error(
      `manifest versions differ: ${TARGETS.map((t) => `${t}=${versions.get(t)}`).join(", ")}`
    );
    process.exit(1);
  }
  return [...unique][0];
}

function assertZipAvailable() {
  try {
    execFileSync("zip", ["-v"], { stdio: "ignore" });
  } catch {
    console.error(
      "The `zip` command is required. Install it (apt-get install zip / brew install zip) and retry."
    );
    process.exit(1);
  }
}

/** Rejects an empty or suspiciously small archive before it reaches a store. */
function assertZipLooksValid(zipPath) {
  if (!existsSync(zipPath) || statSync(zipPath).size < 1024) {
    console.error(`packaging produced an invalid archive: ${zipPath}`);
    process.exit(1);
  }
}

function packageTarget(target, version) {
  console.log(`building ${target}…`);
  execFileSync(process.execPath, [join(ROOT, "scripts", "build.mjs"), target], {
    stdio: "inherit",
  });

  const source = join(DIST, target);
  if (!existsSync(join(source, "manifest.json"))) {
    console.error(`${target} build has no manifest.json`);
    process.exit(1);
  }
  if (readdirSync(join(source, "src")).length === 0) {
    console.error(`${target} build has an empty src/`);
    process.exit(1);
  }

  const zipPath = join(DIST, `sorolens-${version}-${target}.zip`);
  rmSync(zipPath, { force: true });
  // -r recurses, -q keeps CI logs readable, -X drops local file metadata so
  // the archive bytes are reproducible across machines.
  execFileSync("zip", ["-r", "-q", "-X", zipPath, "."], { cwd: source });
  assertZipLooksValid(zipPath);
  console.log(`  ${zipPath}`);
  return zipPath;
}

mkdirSync(DIST, { recursive: true });
assertZipAvailable();
const version = assertVersionsMatch();
for (const target of targets) {
  packageTarget(target, version);
}
console.log(`packaged ${targets.join(" + ")} at version ${version}`);
