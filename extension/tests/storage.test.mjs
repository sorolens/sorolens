import test from "node:test";
import assert from "node:assert/strict";

import {
  DEFAULT_SETTINGS,
  baseUrlToOriginPattern,
  clearApiKey,
  ensureUserId,
  generateUserId,
  getSettings,
  normalizeBaseUrl,
  readFreshEntry,
  readStatusCache,
  saveSettings,
  writeStatusCache,
} from "../src/shared/storage.js";
import { DEFAULT_API_BASE_URL, LOCAL_KEYS, STATUS_CACHE_MAX_ENTRIES } from "../src/shared/constants.js";
import { CONTRACT_ID, installFakeBrowser } from "./fixtures.mjs";

test("normalizeBaseUrl trims whitespace and trailing slashes", () => {
  assert.equal(normalizeBaseUrl(" https://api.sorolens.xyz/ "), "https://api.sorolens.xyz");
  assert.equal(normalizeBaseUrl("https://api.sorolens.xyz///"), "https://api.sorolens.xyz");
  assert.equal(normalizeBaseUrl("   "), "");
  assert.equal(normalizeBaseUrl(undefined), "");
});

test("baseUrlToOriginPattern only accepts http(s) origins", () => {
  assert.equal(baseUrlToOriginPattern("https://api.sorolens.xyz"), "https://api.sorolens.xyz/*");
  assert.equal(baseUrlToOriginPattern("http://localhost:8080/"), "http://localhost:8080/*");
  assert.equal(baseUrlToOriginPattern("ftp://example.com"), "");
  assert.equal(baseUrlToOriginPattern("not a url"), "");
});

test("generateUserId matches the dashboard identity format", () => {
  assert.match(generateUserId(), /^user_\d+_[a-z0-9]{1,10}$/);
});

test("getSettings falls back to the defaults", async () => {
  const browser = installFakeBrowser();
  try {
    const settings = await getSettings();
    assert.deepEqual(settings, DEFAULT_SETTINGS);
  } finally {
    browser.restore();
  }
});

test("saveSettings keeps the API key in local storage and prefs in sync", async () => {
  const browser = installFakeBrowser();
  try {
    const saved = await saveSettings({
      apiKey: "  sl_test_key  ",
      apiBaseUrl: "https://api.example.test/",
      network: "testnet",
      userId: "user_42",
      showInlineButtons: false,
      trackWatchlist: false,
    });

    assert.equal(saved.apiKey, "sl_test_key");
    assert.equal(saved.apiBaseUrl, "https://api.example.test");
    assert.equal(saved.network, "testnet");
    assert.equal(saved.showInlineButtons, false);
    // The secret must never land in the synced area.
    assert.equal(browser.local.get(LOCAL_KEYS.apiKey), "sl_test_key");
    for (const value of browser.sync.values()) {
      assert.notEqual(value, "sl_test_key");
    }
  } finally {
    browser.restore();
  }
});

test("saveSettings ignores an unknown network and empty base URL", async () => {
  const browser = installFakeBrowser();
  try {
    const saved = await saveSettings({ network: "regtest", apiBaseUrl: "" });
    assert.equal(saved.network, DEFAULT_SETTINGS.network);
    assert.equal(saved.apiBaseUrl, DEFAULT_API_BASE_URL);
  } finally {
    browser.restore();
  }
});

test("ensureUserId generates once and then reuses the stored identity", async () => {
  const browser = installFakeBrowser();
  try {
    const first = await ensureUserId();
    const second = await ensureUserId();
    assert.match(first, /^user_/);
    assert.equal(first, second);
  } finally {
    browser.restore();
  }
});

test("clearApiKey removes only the secret", async () => {
  const browser = installFakeBrowser();
  try {
    await saveSettings({ apiKey: "sl_key", userId: "user_keep" });
    await clearApiKey();
    const settings = await getSettings();
    assert.equal(settings.apiKey, "");
    assert.equal(settings.userId, "user_keep");
  } finally {
    browser.restore();
  }
});

test("readFreshEntry honours the TTL", () => {
  const cache = {
    [CONTRACT_ID]: {
      contractId: CONTRACT_ID,
      tracked: true,
      inWatchlist: false,
      checkedAt: 1_000,
    },
  };
  assert.ok(readFreshEntry(cache, CONTRACT_ID, 1_000 + 30_000));
  assert.equal(readFreshEntry(cache, CONTRACT_ID, 1_000 + 120_000), null);
  assert.equal(readFreshEntry(cache, "missing", 1_000), null);
});

test("writeStatusCache merges entries and trims the oldest beyond the cap", async () => {
  const browser = installFakeBrowser();
  try {
    const cache = await readStatusCache();
    assert.deepEqual(cache, {});

    const filled = await writeStatusCache(
      cache,
      Array.from({ length: STATUS_CACHE_MAX_ENTRIES + 10 }, (_value, index) => ({
        contractId: `C${String(index).padStart(55, "0")}`,
        tracked: false,
        inWatchlist: false,
        checkedAt: index,
      })),
    );
    assert.equal(Object.keys(filled).length, STATUS_CACHE_MAX_ENTRIES);
    // The ten oldest (lowest checkedAt) are the ones dropped.
    assert.equal(filled["C" + String(0).padStart(55, "0")], undefined);
    assert.ok(filled["C" + String(STATUS_CACHE_MAX_ENTRIES + 9).padStart(55, "0")]);
  } finally {
    browser.restore();
  }
});
