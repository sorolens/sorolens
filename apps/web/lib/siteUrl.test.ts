import { afterEach, describe, expect, it, vi } from "vitest";
import { absoluteUrl, siteOrigin } from "./siteUrl";

describe("siteOrigin", () => {
  afterEach(() => vi.unstubAllEnvs());

  it("uses NEXT_PUBLIC_APP_URL when set", () => {
    vi.stubEnv("NEXT_PUBLIC_APP_URL", "https://staging.sorolens.xyz");
    expect(siteOrigin()).toBe("https://staging.sorolens.xyz");
  });

  it("strips a single trailing slash", () => {
    vi.stubEnv("NEXT_PUBLIC_APP_URL", "https://staging.sorolens.xyz/");
    expect(siteOrigin()).toBe("https://staging.sorolens.xyz");
  });

  it("falls back to the production origin when unset", () => {
    vi.stubEnv("NEXT_PUBLIC_APP_URL", undefined);
    expect(siteOrigin()).toBe("https://sorolens.xyz");
  });

  it("ignores a blank value", () => {
    vi.stubEnv("NEXT_PUBLIC_APP_URL", "   ");
    expect(siteOrigin()).toBe("https://sorolens.xyz");
  });
});

describe("absoluteUrl", () => {
  afterEach(() => vi.unstubAllEnvs());

  it("joins a path to the origin", () => {
    vi.stubEnv("NEXT_PUBLIC_APP_URL", "https://sorolens.xyz");
    expect(absoluteUrl("/contracts")).toBe("https://sorolens.xyz/contracts");
  });

  it("adds a missing leading slash", () => {
    vi.stubEnv("NEXT_PUBLIC_APP_URL", "https://sorolens.xyz");
    expect(absoluteUrl("events")).toBe("https://sorolens.xyz/events");
  });

  it("names the site root with its trailing slash", () => {
    vi.stubEnv("NEXT_PUBLIC_APP_URL", "https://sorolens.xyz");
    expect(absoluteUrl("/")).toBe("https://sorolens.xyz/");
    expect(absoluteUrl()).toBe("https://sorolens.xyz/");
  });

  it("never produces a double slash", () => {
    vi.stubEnv("NEXT_PUBLIC_APP_URL", "https://sorolens.xyz/");
    expect(absoluteUrl("/status")).toBe("https://sorolens.xyz/status");
    expect(absoluteUrl("//nested//path")).toBe(
      "https://sorolens.xyz/nested/path"
    );
  });
});
