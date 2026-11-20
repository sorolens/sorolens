import { describe, expect, it } from "vitest";

import {
  DEFAULT_SETTINGS,
  SETTINGS_STORAGE_KEY,
  clearSettings,
  isValidEmail,
  loadSettings,
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
  });

  it("drops a legacy plaintext apiKey from an older blob", () => {
    const storage = memoryStorage();
    storage.setItem(
      SETTINGS_STORAGE_KEY,
      JSON.stringify({
        defaultNetwork: "mainnet",
        apiKey: "sl_live_fabricated",
        apiKeyCreatedAt: "2026-09-25T00:00:00.000Z",
      })
    );

    const loaded = loadSettings(storage) as unknown as Record<string, unknown>;
    expect(loaded.apiKey).toBeUndefined();
    expect(loaded.apiKeyCreatedAt).toBeUndefined();
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

describe("unavailable storage", () => {
  /** A Storage whose reads fail, as when the browser blocks site data. */
  function throwingStorage(): Storage {
    const storage = memoryStorage();
    return {
      ...storage,
      getItem: () => {
        throw new Error("Storage is blocked");
      },
      setItem: () => {
        throw new Error("QuotaExceededError");
      },
      removeItem: () => {
        throw new Error("Storage is blocked");
      },
    } as Storage;
  }

  it("loadSettings falls back to defaults when a read throws", () => {
    expect(loadSettings(throwingStorage())).toEqual(DEFAULT_SETTINGS);
  });

  it("saveSettings reports failure when a write throws", () => {
    expect(saveSettings(DEFAULT_SETTINGS, throwingStorage())).toBe(false);
  });

  it("clearSettings swallows a failing removal", () => {
    expect(() => clearSettings(throwingStorage())).not.toThrow();
  });

  it("all helpers fall back safely when localStorage itself throws", () => {
    const descriptor = Object.getOwnPropertyDescriptor(
      globalThis,
      "localStorage"
    );
    Object.defineProperty(globalThis, "localStorage", {
      configurable: true,
      get() {
        throw new Error("Access is denied for this document");
      },
    });

    try {
      expect(loadSettings()).toEqual(DEFAULT_SETTINGS);
      expect(saveSettings(DEFAULT_SETTINGS)).toBe(false);
      expect(() => clearSettings()).not.toThrow();
    } finally {
      if (descriptor) {
        Object.defineProperty(globalThis, "localStorage", descriptor);
      }
    }
  });
});
