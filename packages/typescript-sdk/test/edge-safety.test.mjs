import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

const ESM = readFileSync(new URL("../dist/esm/index.js", import.meta.url), "utf8");
const CJS = readFileSync(new URL("../dist/cjs/index.cjs", import.meta.url), "utf8");

test("the ESM bundle imports no Node built-ins", () => {
  assert.doesNotMatch(ESM, /from\s+["']node:/);
  assert.doesNotMatch(ESM, /require\(\s*["']node:/);
  assert.doesNotMatch(ESM, /\bBuffer\b/);
  assert.doesNotMatch(ESM, /\bprocess\./);
});

test("the CJS bundle does not pull in Node modules", () => {
  assert.doesNotMatch(CJS, /require\(\s*["']node:/);
});

test("the package is marked side-effect free for tree-shaking", () => {
  const pkg = JSON.parse(readFileSync(new URL("../package.json", import.meta.url), "utf8"));
  assert.equal(pkg.sideEffects, false);
  assert.equal(pkg.exports["."].import, "./dist/esm/index.js");
  assert.equal(pkg.exports["."].require, "./dist/cjs/index.cjs");
  assert.equal(pkg.exports["."].types, "./dist/types/index.d.ts");
});
