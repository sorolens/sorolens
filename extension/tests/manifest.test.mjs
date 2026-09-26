import test from "node:test";
import assert from "node:assert/strict";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const EXPLORER_MATCHES = [
  "https://stellar.expert/*",
  "https://lab.stellar.org/*",
  "https://stellar.org/*",
];

function readManifest(name) {
  return JSON.parse(readFileSync(join(ROOT, name), "utf8"));
}

test("both manifests are Manifest V3 with the required metadata", () => {
  for (const name of ["manifest.json", "manifest.firefox.json"]) {
    const manifest = readManifest(name);
    assert.equal(manifest.manifest_version, 3, `${name} must be MV3`);
    assert.ok(manifest.name);
    assert.ok(manifest.version);
    assert.ok(manifest.description);
    assert.ok(manifest.action.default_popup);
    assert.ok(manifest.options_ui.page);
  }
});

test("Chrome uses a module service worker, Firefox uses a module event page", () => {
  const chrome = readManifest("manifest.json");
  assert.equal(chrome.background.type, "module");
  assert.match(chrome.background.service_worker, /service-worker\.js$/);
  assert.equal(chrome.background.scripts, undefined);

  const firefox = readManifest("manifest.firefox.json");
  assert.equal(firefox.background.type, "module");
  assert.deepEqual(firefox.background.scripts, ["src/background/service-worker.js"]);
  assert.match(firefox.browser_specific_settings.gecko.id, /@/);
});

test("content scripts run on the three explorer origins", () => {
  for (const name of ["manifest.json", "manifest.firefox.json"]) {
    const manifest = readManifest(name);
    const matches = new Set(
      manifest.content_scripts.flatMap((script) => script.matches),
    );
    for (const origin of EXPLORER_MATCHES) {
      assert.ok(matches.has(origin), `${name} is missing ${origin}`);
    }
    const js = manifest.content_scripts.flatMap((script) => script.js);
    assert.deepEqual(js, ["src/content/loader.js"]);
    const css = manifest.content_scripts.flatMap((script) => script.css);
    assert.deepEqual(css, ["src/content/content.css"]);
  }
});

test("host permissions cover the API and the explorer origins", () => {
  for (const name of ["manifest.json", "manifest.firefox.json"]) {
    const manifest = readManifest(name);
    for (const origin of ["https://api.sorolens.xyz/*", ...EXPLORER_MATCHES]) {
      assert.ok(
        manifest.host_permissions.includes(origin),
        `${name} is missing host permission ${origin}`,
      );
    }
  }
});

test("every file referenced by the manifests exists", () => {
  for (const name of ["manifest.json", "manifest.firefox.json"]) {
    const manifest = readManifest(name);
    const referenced = [
      ...Object.values(manifest.icons),
      ...Object.values(manifest.action.default_icon),
      manifest.action.default_popup,
      manifest.options_ui.page,
      manifest.background.service_worker,
      ...(manifest.background.scripts || []),
      ...manifest.content_scripts.flatMap((script) => [
        ...script.js,
        ...script.css,
      ]),
    ].filter(Boolean);
    for (const file of referenced) {
      assert.ok(existsSync(join(ROOT, file)), `${name} references missing ${file}`);
    }
  }
});

test("web accessible resources cover the dynamic content-script imports", () => {
  for (const name of ["manifest.json", "manifest.firefox.json"]) {
    const manifest = readManifest(name);
    const blocked = new Set(
      manifest.content_scripts.flatMap((script) => script.matches),
    );
    const sharedModules = readdirSync(join(ROOT, "src/shared")).filter((file) =>
      file.endsWith(".js"),
    );
    assert.ok(sharedModules.length > 0);
    for (const entry of manifest.web_accessible_resources) {
      for (const match of entry.matches) {
        assert.ok(blocked.has(match), `${name}: ${match} is not a content-script origin`);
      }
    }
    const patterns = manifest.web_accessible_resources.flatMap(
      (entry) => entry.resources,
    );
    assert.ok(patterns.includes("src/content/*.js"));
    assert.ok(patterns.includes("src/shared/*.js"));
  }
});

test("the extension declares no remote code or eval", () => {
  for (const name of ["manifest.json", "manifest.firefox.json"]) {
    const manifest = readManifest(name);
    assert.equal(manifest.content_security_policy, undefined);
    assert.equal(manifest.externally_connectable, undefined);
  }
  const modules = [
    "src/background/service-worker.js",
    "src/content/content.js",
    "src/shared/api.js",
  ];
  for (const module of modules) {
    const source = readFileSync(join(ROOT, module), "utf8");
    assert.equal(/\beval\s*\(/.test(source), false, `${module} uses eval()`);
    assert.equal(
      /\bnew Function\s*\(/.test(source),
      false,
      `${module} uses new Function()`,
    );
    assert.equal(
      /<script[^>]+src=["']https?:/.test(source),
      false,
      `${module} loads remote script`,
    );
  }
});
