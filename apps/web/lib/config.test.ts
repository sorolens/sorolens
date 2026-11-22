/** Unit tests for the API base URL in apps/web/lib/config.ts. */

import { afterEach, describe, expect, it, vi } from "vitest";

describe("config API_URL", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
    vi.resetModules();
  });

  it("falls back to the local API when NEXT_PUBLIC_API_URL is unset", async () => {
    vi.stubEnv("NEXT_PUBLIC_API_URL", "");
    const { API_URL } = await import("./config");
    expect(API_URL).toBe("http://localhost:8080");
  });

  it("uses NEXT_PUBLIC_API_URL when set", async () => {
    vi.stubEnv("NEXT_PUBLIC_API_URL", "https://api.example.com");
    const { API_URL } = await import("./config");
    expect(API_URL).toBe("https://api.example.com");
  });
});
