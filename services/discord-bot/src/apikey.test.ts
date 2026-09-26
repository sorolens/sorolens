import { describe, it, expect } from "vitest";
import {
  ensureUserApiKey,
  getUserApiKey,
  keyNameFor,
  mintApiKey,
  revokeUserApiKey,
  type ApiKeyMinter,
  type ApiKeyStore,
  type StoredApiKey,
} from "./apikey.js";

function memStore(): ApiKeyStore & { size(): number } {
  const m = new Map<string, StoredApiKey>();
  return {
    get: (discordId) => m.get(discordId) ?? null,
    set: (discordId, key, keyId) => {
      const row: StoredApiKey = { discordId, key, keyId, createdAt: "2026-01-01 00:00:00" };
      m.set(discordId, row);
      return row;
    },
    clear: (discordId) => m.delete(discordId),
    size: () => m.size,
  };
}

function recordingMinter(): ApiKeyMinter & { provisioned: string[]; revoked: string[] } {
  const provisioned: string[] = [];
  const revoked: string[] = [];
  return {
    provisioned,
    revoked,
    async provisionApiKey(name) {
      provisioned.push(name);
      return { id: `key_${provisioned.length}`, key: `sl_${provisioned.length}` };
    },
    async revokeApiKey(id) {
      revoked.push(id);
    },
  };
}

describe("mintApiKey", () => {
  it("produces an sl_-prefixed key", () => {
    expect(mintApiKey().startsWith("sl_")).toBe(true);
  });

  it("produces distinct keys", () => {
    expect(mintApiKey()).not.toBe(mintApiKey());
  });
});

describe("keyNameFor", () => {
  it("encodes the discord id and login for auditability", () => {
    expect(keyNameFor("42", "alice")).toBe("discord:42:alice");
  });
});

describe("ensureUserApiKey", () => {
  it("provisions and stores a key on first call", async () => {
    const store = memStore();
    const minter = recordingMinter();

    const granted = await ensureUserApiKey(store, minter, "42", "alice");

    expect(granted).not.toBeNull();
    expect(granted?.key).toBe("sl_1");
    expect(granted?.keyId).toBe("key_1");
    expect(minter.provisioned).toEqual(["discord:42:alice"]);
    expect(getUserApiKey(store, "42")?.keyId).toBe("key_1");
  });

  it("is idempotent and never provisions twice", async () => {
    const store = memStore();
    const minter = recordingMinter();

    await ensureUserApiKey(store, minter, "42", "alice");
    const again = await ensureUserApiKey(store, minter, "42", "alice");

    expect(again?.key).toBe("sl_1");
    expect(minter.provisioned).toHaveLength(1);
    expect(store.size()).toBe(1);
  });

  it("swallows provisioning failures so linking still succeeds", async () => {
    const store = memStore();
    const minter: ApiKeyMinter = {
      async provisionApiKey() {
        throw new Error("503 from upstream");
      },
      async revokeApiKey() {},
    };

    await expect(ensureUserApiKey(store, minter, "42", "alice")).resolves.toBeNull();
    expect(getUserApiKey(store, "42")).toBeNull();
  });
});

describe("revokeUserApiKey", () => {
  it("revokes upstream and clears the local row", async () => {
    const store = memStore();
    const minter = recordingMinter();
    await ensureUserApiKey(store, minter, "42", "alice");

    const result = await revokeUserApiKey(store, minter, "42");

    expect(result).toEqual({ revoked: true, cleared: true });
    expect(minter.revoked).toEqual(["key_1"]);
    expect(getUserApiKey(store, "42")).toBeNull();
  });

  it("clears locally even when revocation fails", async () => {
    const store = memStore();
    const minter: ApiKeyMinter = {
      async provisionApiKey() {
        return { id: "key_9", key: "sl_9" };
      },
      async revokeApiKey() {
        throw new Error("network down");
      },
    };
    await ensureUserApiKey(store, minter, "42", "alice");

    const result = await revokeUserApiKey(store, minter, "42");

    expect(result).toEqual({ revoked: false, cleared: true });
    expect(getUserApiKey(store, "42")).toBeNull();
  });

  it("is a no-op when the user has no key", async () => {
    const store = memStore();
    const minter = recordingMinter();
    const result = await revokeUserApiKey(store, minter, "nobody");
    expect(result).toEqual({ revoked: false, cleared: false });
    expect(minter.revoked).toEqual([]);
  });
});
