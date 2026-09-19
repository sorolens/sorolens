import { describe, it, expect, beforeEach } from "vitest";
import { rmSync, mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { openDb, upsertLink, unlink, getByDiscord, getByGithub } from "./db.js";

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
});
