import { describe, it, expect, beforeEach } from "vitest";
import { rmSync, mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import {
  openDb,
  upsertLink,
  unlink,
  getByDiscord,
  getByGithub,
  setApiKey,
  getApiKey,
  clearApiKey,
  upsertSubscription,
  removeSubscription,
  getSubscription,
  listSubscriptionsForChannel,
  listSubscriptionsForContract,
  clearSubscriptionsForChannel,
} from "./db.js";

describe("db mapping store", () => {
  let dir: string;
  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), "sorolens-bot-"));
    openDb(join(dir, "test.db"));
  });

  it("stores a new link and reads it back", () => {
    upsertLink("111", "alice");
    expect(getByDiscord("111")?.githubLogin).toBe("alice");
    expect(getByGithub("alice")?.discordId).toBe("111");
  });

  it("is case-insensitive on GitHub login", () => {
    upsertLink("111", "Alice");
    expect(getByGithub("alice")?.githubLogin).toBe("Alice");
  });

  it("upsert replaces the same user's link", () => {
    upsertLink("111", "alice");
    upsertLink("111", "alicia");
    expect(getByDiscord("111")?.githubLogin).toBe("alicia");
    expect(getByGithub("alice")).toBeNull();
  });

  it("prevents two users claiming the same GitHub name", () => {
    upsertLink("111", "alice");
    expect(() => upsertLink("222", "alice")).toThrow(/already linked/);
  });

  it("unlink removes the row", () => {
    upsertLink("111", "alice");
    expect(unlink("111")).toBe(true);
    expect(getByDiscord("111")).toBeNull();
  });

  it("unlink returns false when nothing to remove", () => {
    expect(unlink("999")).toBe(false);
  });

  it("unlink also drops the stored API key", () => {
    upsertLink("111", "alice");
    setApiKey("111", "sl_secret", "key_1");
    expect(getApiKey("111")?.key).toBe("sl_secret");
    expect(unlink("111")).toBe(true);
    expect(getApiKey("111")).toBeNull();
  });
});

describe("per-user api key store", () => {
  let dir: string;
  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), "sorolens-bot-"));
    openDb(join(dir, "test.db"));
  });

  it("stores and reads back a key", () => {
    setApiKey("111", "sl_abc", "key_1");
    const row = getApiKey("111");
    expect(row?.key).toBe("sl_abc");
    expect(row?.keyId).toBe("key_1");
    expect(row?.createdAt).toBeTruthy();
  });

  it("returns null for an unknown user", () => {
    expect(getApiKey("nobody")).toBeNull();
  });

  it("upsert replaces the key for the same user", () => {
    setApiKey("111", "sl_old", "key_1");
    setApiKey("111", "sl_new", "key_2");
    expect(getApiKey("111")?.key).toBe("sl_new");
    expect(getApiKey("111")?.keyId).toBe("key_2");
  });

  it("clear reports whether a row was removed", () => {
    setApiKey("111", "sl_abc", "key_1");
    expect(clearApiKey("111")).toBe(true);
    expect(clearApiKey("111")).toBe(false);
  });
});

describe("alert subscriptions store", () => {
  let dir: string;
  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), "sorolens-bot-"));
    openDb(join(dir, "test.db"));
  });

  const CONTRACT_A = "C" + "A".repeat(55);
  const CONTRACT_B = "C" + "B".repeat(55);

  it("stores and reads back a subscription", () => {
    const sub = upsertSubscription("chan_1", CONTRACT_A, "critical");
    expect(sub.channelId).toBe("chan_1");
    expect(sub.contractId).toBe(CONTRACT_A);
    expect(sub.severity).toBe("critical");
    expect(sub.createdAt).toBeTruthy();
    expect(sub.updatedAt).toBeTruthy();

    const read = getSubscription("chan_1", CONTRACT_A);
    expect(read).toEqual(sub);
  });

  it("normalizes contract to uppercase and severity to lowercase", () => {
    upsertSubscription("chan_1", CONTRACT_A.toLowerCase(), "WARNING");
    const read = getSubscription("chan_1", CONTRACT_A);
    expect(read?.contractId).toBe(CONTRACT_A);
    expect(read?.severity).toBe("warning");
  });

  it("upsert replaces severity for the same channel and contract", () => {
    upsertSubscription("chan_1", CONTRACT_A, "all");
    expect(getSubscription("chan_1", CONTRACT_A)?.severity).toBe("all");

    upsertSubscription("chan_1", CONTRACT_A, "critical");
    expect(getSubscription("chan_1", CONTRACT_A)?.severity).toBe("critical");
  });

  it("removeSubscription removes the row and reports boolean", () => {
    upsertSubscription("chan_1", CONTRACT_A, "all");
    expect(removeSubscription("chan_1", CONTRACT_A)).toBe(true);
    expect(getSubscription("chan_1", CONTRACT_A)).toBeNull();
    expect(removeSubscription("chan_1", CONTRACT_A)).toBe(false);
  });

  it("lists subscriptions by channel and contract", () => {
    upsertSubscription("chan_1", CONTRACT_A, "critical");
    upsertSubscription("chan_1", CONTRACT_B, "all");
    upsertSubscription("chan_2", CONTRACT_A, "warning");

    const chan1Subs = listSubscriptionsForChannel("chan_1");
    expect(chan1Subs).toHaveLength(2);
    expect(chan1Subs.map((s) => s.contractId)).toEqual([CONTRACT_A, CONTRACT_B]);

    const contractASubs = listSubscriptionsForContract(CONTRACT_A);
    expect(contractASubs).toHaveLength(2);
    expect(contractASubs.map((s) => s.channelId)).toEqual(["chan_1", "chan_2"]);
  });

  it("clearSubscriptionsForChannel removes all subscriptions for a channel", () => {
    upsertSubscription("chan_1", CONTRACT_A, "critical");
    upsertSubscription("chan_1", CONTRACT_B, "all");
    upsertSubscription("chan_2", CONTRACT_A, "warning");

    expect(clearSubscriptionsForChannel("chan_1")).toBe(2);
    expect(listSubscriptionsForChannel("chan_1")).toEqual([]);
    expect(listSubscriptionsForChannel("chan_2")).toHaveLength(1);
  });
});
