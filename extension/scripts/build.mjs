// Zero-dependency packaging: produce loadable (unpacked) Chrome and Firefox
// builds from one source tree.
//
// The extension loads unpacked straight from `extension/` with no build step
// at all; this script only exists to emit the two manifest variants side by
// side, which is what "one codebase, two stores" needs at submission time.
//
// Usage: node scripts/build.mjs <chrome|firefox>

import { cpSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const DIST = join(ROOT, "dist");

/** target -> manifest file to use as manifest.json */
const TARGETS = {
  chrome: "manifest.json",
  firefox: "manifest.firefox.json",
};

const target = process.argv[2];
if (!target || !(target in TARGETS)) {
  console.error(`Usage: node scripts/build.mjs <${Object.keys(TARGETS).join("|")}>`);
  process.exit(1);
}

const outDir = join(DIST, target);
const manifestSource = join(ROOT, TARGETS[target]);

// Fail loudly instead of shipping a half-built bundle.
const manifest = JSON.parse(readFileSync(manifestSource, "utf8"));
if (manifest.manifest_version !== 3) {
  console.error(`${TARGETS[target]} is not a Manifest V3 manifest`);
  process.exit(1);
}

rmSync(outDir, { recursive: true, force: true });
mkdirSync(outDir, { recursive: true });
cpSync(join(ROOT, "src"), join(outDir, "src"), { recursive: true });
writeFileSync(
  join(outDir, "manifest.json"),
  `${JSON.stringify(manifest, null, 2)}\n`,
);

console.log(`built ${target} -> ${outDir}`);
console.log(`  manifest: ${TARGETS[target]}`);
console.log(`  load unpacked from that directory`);
