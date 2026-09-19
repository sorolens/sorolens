import { describe, it, expect } from "vitest";
import { buildConnectUrl, signDiscordId } from "./oauth.js";
import type { Config } from "./config.js";

const cfg: Config = {
  discordToken: "x",
  discordClientId: "x",
  discordGuildId: "guild",
  roleContributor: "rc",
  roleCoreContributor: "rcc",
  roleVerified: "rv",
  contributorThreshold: 1,
  coreContributorThreshold: 3,
  githubToken: "x",
  githubRepo: "sorolens/sorolens",
  githubWebhookSecret: "x",
  githubOauthClientId: "gh_id",
  githubOauthClientSecret: "gh_secret",
  oauthStateSecret: "super-secret-do-not-guess",
  publicBaseUrl: "https://sorolens.onrender.com",
  port: 8080,
  dbPath: "/tmp/x.db",
};

describe("oauth start URL", () => {
  it("carries the caller's discord id and a deterministic signature", () => {
    const url = buildConnectUrl("1111", cfg);
    const expected = signDiscordId("1111", cfg.oauthStateSecret);
    expect(url).toBe(
      `https://sorolens.onrender.com/oauth/start?discord_id=1111&sig=${expected}`,
    );
  });

  it("signatures differ per discord id", () => {
    const a = signDiscordId("aaa", cfg.oauthStateSecret);
    const b = signDiscordId("bbb", cfg.oauthStateSecret);
    expect(a).not.toBe(b);
  });

  it("signatures differ per secret", () => {
    const a = signDiscordId("aaa", "secret-1");
    const b = signDiscordId("aaa", "secret-2");
    expect(a).not.toBe(b);
  });
});
