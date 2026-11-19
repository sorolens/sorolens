import { afterEach, describe, expect, it, vi } from "vitest";
import { absoluteUrl, siteOrigin } from "./siteUrl";

describe("siteOrigin", () => {
  afterEach(() => vi.unstubAllEnvs());

  it("uses NEXT_PUBLIC_APP_URL when set", () => {
    vi.stubEnv("NEXT_PUBLIC_APP_URL", "https://staging.sorolens.dev");
    expect(siteOrigin()).toBe("https://staging.sorolens.dev");
  });

  it("strips a single trailing slash", () => {
    vi.stubEnv("NEXT_PUBLIC_APP_URL", "https://staging.sorolens.dev/");
    expect(siteOrigin()).toBe("https://staging.sorolens.dev");
  });

  it("falls back to the production origin when unset", () => {
    vi.stubEnv("NEXT_PUBLIC_APP_URL", undefined);
    expect(siteOrigin()).toBe("https://sorolens.dev");
  });

  it("ignores a blank value", () => {
    vi.stubEnv("NEXT_PUBLIC_APP_URL", "   ");
    expect(siteOrigin()).toBe("https://sorolens.dev");
  });
});

describe("absoluteUrl", () => {
  afterEach(() => vi.unstubAllEnvs());

  it("joins a path to the origin", () => {
    vi.stubEnv("NEXT_PUBLIC_APP_URL", "https://sorolens.dev");
    expect(absoluteUrl("/contracts")).toBe("https://sorolens.dev/contracts");
  });

  it("adds a missing leading slash", () => {
    vi.stubEnv("NEXT_PUBLIC_APP_URL", "https://sorolens.dev");
    expect(absoluteUrl("events")).toBe("https://sorolens.dev/events");
  });

  it("names the site root with its trailing slash", () => {
    vi.stubEnv("NEXT_PUBLIC_APP_URL", "https://sorolens.dev");
    expect(absoluteUrl("/")).toBe("https://sorolens.dev/");
    expect(absoluteUrl()).toBe("https://sorolens.dev/");
  });

  it("never produces a double slash", () => {
    vi.stubEnv("NEXT_PUBLIC_APP_URL", "https://sorolens.dev/");
    expect(absoluteUrl("/status")).toBe("https://sorolens.dev/status");
    expect(absoluteUrl("//nested//path")).toBe(
      "https://sorolens.dev/nested/path"
    );
  });
});
