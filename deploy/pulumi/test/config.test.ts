import { describe, expect, it } from "vitest";

import { SorolensConfigError, buildAppConfig } from "../src/core/config";

const URL = "https://sorolens.example.com";
const WATCHDOG = "CACXRL67WL5KRD6HKWGYADHEUF6RQOCODUN26UQE7MGFZEMIR7PAX6R7";

describe("buildAppConfig (mirrors terraform modules/core)", () => {
  it("defaults to testnet with the public SDF endpoint", () => {
    const c = buildAppConfig({}, URL);
    expect(c.apiEnvironment.STELLAR_NETWORK).toBe("testnet");
    expect(c.apiEnvironment.SOROBAN_RPC_URL).toBe("https://soroban-testnet.stellar.org:443");
    expect(c.indexerEnvironment.SOROBAN_RPC_URL_TESTNET).toBe("https://soroban-testnet.stellar.org:443");
    expect(c.indexerEnvironment).not.toHaveProperty("SOROBAN_RPC_URL_MAINNET");
    expect(c.indexerEnvironment.INDEXER_ROLE).toBe("all");
    expect(c.indexerEnvironment.WATCHDOG_ENABLED).toBe("false");
    expect(c.indexerEnvironment).not.toHaveProperty("WATCHDOG_CONTRACT_ID");
    expect(c.apiEnvironment).not.toHaveProperty("ALLOWED_ORIGINS");
    expect(c.dashboardEnvironment.NEXT_PUBLIC_API_URL).toBe(URL);
    expect(c.indexerCommand).toEqual(["-mode=continuous", "-poll-interval=5m"]);
  });

  it("never puts secrets in the plain environment", () => {
    const c = buildAppConfig({}, URL);
    for (const env of [c.apiEnvironment, c.indexerEnvironment, c.dashboardEnvironment]) {
      for (const secret of ["DATABASE_URL", "DIRECT_DATABASE_URL", "REDIS_URL", "SENTRY_DSN"]) {
        expect(env).not.toHaveProperty(secret);
      }
    }
  });

  it("indexes every configured network and applies overrides", () => {
    const c = buildAppConfig(
      {
        stellarNetwork: "mainnet",
        sorobanRpcUrls: { mainnet: "https://rpc.example.com", testnet: "https://testnet-rpc.example.com" },
        watchdogContractId: WATCHDOG,
        allowedOrigins: ["https://a.example.com", "https://b.example.com"],
        indexerPollInterval: "30s",
      },
      URL,
    );
    expect(c.apiEnvironment.SOROBAN_RPC_URL).toBe("https://rpc.example.com");
    expect(c.indexerEnvironment.SOROBAN_RPC_URL_MAINNET).toBe("https://rpc.example.com");
    expect(c.indexerEnvironment.SOROBAN_RPC_URL_TESTNET).toBe("https://testnet-rpc.example.com");
    expect(c.indexerEnvironment.WATCHDOG_ENABLED).toBe("true");
    expect(c.indexerEnvironment.WATCHDOG_CONTRACT_ID).toBe(WATCHDOG);
    expect(c.apiEnvironment.ALLOWED_ORIGINS).toBe("https://a.example.com,https://b.example.com");
    expect(c.indexerCommand[1]).toBe("-poll-interval=30s");
  });

  it.each([
    ["mainnet without an RPC endpoint", { stellarNetwork: "mainnet" as const }],
    ["an unknown network", { stellarNetwork: "pubnet" as never }],
    ["a plain-http RPC endpoint", { sorobanRpcUrls: { testnet: "http://insecure.example.com" } }],
    ["a malformed watchdog id", { watchdogContractId: "not-a-contract" }],
    ["a name that is too long", { name: "this-name-is-far-too-long-for-aws" }],
    ["a name ending in a hyphen", { name: "sorolens-" }],
    ["an origin with a path", { allowedOrigins: ["https://example.com/app"] }],
    ["a bad poll interval", { indexerPollInterval: "five minutes" }],
  ])("rejects %s", (_label, args) => {
    expect(() => buildAppConfig(args, URL)).toThrow(SorolensConfigError);
  });
});
