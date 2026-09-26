import { describe, expect, it } from "vitest";

import {
  DEFAULT_SETTINGS,
  SETTINGS_STORAGE_KEY,
  clearSettings,
  generateApiKey,
  isValidEmail,
  loadSettings,
  maskApiKey,
  saveSettings,
  type UserSettings,
} from "./settings";

/** Minimal in-memory Storage so these tests don't depend on jsdom. */
function memoryStorage(): Storage {
  const map = new Map<string, string>();
  return {
    get length() {
      return map.size;
    },
    clear: () => map.clear(),
    getItem: (key: string) => (map.has(key) ? (map.get(key) as string) : null),
    key: (index: number) => Array.from(map.keys())[index] ?? null,
    removeItem: (key: string) => {
      map.delete(key);
    },
    setItem: (key: string, value: string) => {
      map.set(key, value);
    },
  };
}

describe("isValidEmail", () => {
  it("accepts ordinary addresses", () => {
    expect(isValidEmail("dev@example.com")).toBe(true);
    expect(isValidEmail("first.last+tag@sorolens.dev")).toBe(true);
  });

  it("rejects malformed addresses", () => {
    expect(isValidEmail("")).toBe(false);
    expect(isValidEmail("not-an-email")).toBe(false);
    expect(isValidEmail("missing@tld")).toBe(false);
    expect(isValidEmail("@example.com")).toBe(false);
    expect(isValidEmail("has space@example.com")).toBe(false);
    expect(isValidEmail("trailing@example.")).toBe(false);
  });
});

describe("generateApiKey", () => {
  it("returns a prefixed hex token of fixed length", () => {
    const key = generateApiKey();
    expect(key.startsWith("sl_live_")).toBe(true);
    expect(key.slice("sl_live_".length)).toMatch(/^[0-9a-f]{48}$/);
  });

  it("does not repeat between calls", () => {
    expect(generateApiKey()).not.toBe(generateApiKey());
  });
});

describe("maskApiKey", () => {
  it("keeps a recognisable prefix and suffix", () => {
    const key = "sl_live_00112233445566778899aabbccddeeff00112233445566";
    const masked = maskApiKey(key);
    expect(masked.startsWith("sl_live_")).toBe(true);
    expect(masked.endsWith(key.slice(-4))).toBe(true);
    expect(masked).not.toBe(key);
  });

  it("returns an empty string when there is no key", () => {
    expect(maskApiKey(null)).toBe("");
  });

  it("fully hides a short value", () => {
    expect(maskApiKey("short")).toBe("•••••");
  });
});

describe("loadSettings / saveSettings", () => {
  it("returns defaults when nothing is stored", () => {
    expect(loadSettings(memoryStorage())).toEqual(DEFAULT_SETTINGS);
  });

  it("round-trips a saved settings object", () => {
    const storage = memoryStorage();
    const settings: UserSettings = {
      defaultNetwork: "mainnet",
      notifications: {
        email: "dev@example.com",
        channels: {
          critical_alerts: false,
          health_changes: true,
          weekly_digest: true,
        },
      },
      apiKey: "sl_live_abc",
      apiKeyCreatedAt: "2026-09-25T00:00:00.000Z",
    };

    expect(saveSettings(settings, storage)).toBe(true);
    expect(loadSettings(storage)).toEqual(settings);
  });

  it("fills missing fields from defaults for older or partial blobs", () => {
    const storage = memoryStorage();
    storage.setItem(
      SETTINGS_STORAGE_KEY,
      JSON.stringify({
        defaultNetwork: "futurenet",
        notifications: { email: "a@b.co" },
      })
    );

    const loaded = loadSettings(storage);
    expect(loaded.defaultNetwork).toBe("futurenet");
    expect(loaded.notifications.email).toBe("a@b.co");
    expect(loaded.notifications.channels).toEqual(
      DEFAULT_SETTINGS.notifications.channels
    );
    expect(loaded.apiKey).toBeNull();
  });

  it("falls back to defaults when the stored blob is corrupt", () => {
    const storage = memoryStorage();
    storage.setItem(SETTINGS_STORAGE_KEY, "{not json");
    expect(loadSettings(storage)).toEqual(DEFAULT_SETTINGS);
  });

  it("clearSettings removes the stored blob", () => {
    const storage = memoryStorage();
    saveSettings(DEFAULT_SETTINGS, storage);
    clearSettings(storage);
    expect(storage.getItem(SETTINGS_STORAGE_KEY)).toBeNull();
  });
});
