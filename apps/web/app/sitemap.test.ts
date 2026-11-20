import type { MetadataRoute } from "next";
import { afterEach, describe, expect, it, vi } from "vitest";
import sitemap from "@/app/sitemap";

const ORIGIN = "https://staging.sorolens.dev";

/** An entry's timestamp; `sitemap.xml` must carry real `Date`s, not strings. */
function stampOf(entry: MetadataRoute.Sitemap[number]): number {
  const { lastModified } = entry;
  if (!(lastModified instanceof Date)) {
    throw new Error(
      `expected a Date lastModified for ${entry.url}, got ${String(lastModified)}`
    );
  }
  return lastModified.getTime();
}

/** One contracts page; `next_cursor: ""` means the sitemap fetches one page. */
function stubContracts(contracts: unknown[], nextCursor = "") {
  const fetchMock = vi.fn(async (input: unknown) => {
    const url = String(input);
    if (url.includes("/api/v1/contracts")) {
      return {
        ok: true,
        json: async () => ({ contracts, next_cursor: nextCursor }),
      };
    }
    return { ok: false, status: 404, json: async () => ({}) };
  });
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

describe("sitemap", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.unstubAllEnvs();
  });

  it("publishes every entry under the configured origin", async () => {
    vi.stubEnv("NEXT_PUBLIC_APP_URL", `${ORIGIN}/`);
    stubContracts([]);

    const entries = await sitemap();

    expect(entries.length).toBeGreaterThan(0);
    for (const entry of entries) {
      expect(entry.url.startsWith(`${ORIGIN}/`)).toBe(true);
      expect(entry.url).not.toContain("localhost");
      // A double slash anywhere past the scheme means an origin/path seam was
      // joined wrong.
      expect(entry.url.replace(/^https?:\/\//, "")).not.toContain("//");
    }
  });

  it("lists every public route named in issue #761", async () => {
    vi.stubEnv("NEXT_PUBLIC_APP_URL", ORIGIN);
    stubContracts([]);

    const paths = (await sitemap()).map((entry) =>
      entry.url.slice(ORIGIN.length)
    );

    for (const expected of [
      "/",
      "/contracts",
      "/events",
      "/events/live",
      "/invocations",
      "/compare",
      "/playground",
      "/watchdog",
      "/watchdog/reports",
    ]) {
      expect(paths).toContain(expected);
    }
  });

  it("omits personal and transactional routes", async () => {
    vi.stubEnv("NEXT_PUBLIC_APP_URL", ORIGIN);
    stubContracts([]);

    const paths = (await sitemap()).map((entry) =>
      entry.url.slice(ORIGIN.length)
    );

    expect(paths).not.toContain("/settings");
    expect(paths).not.toContain("/dashboard");
    expect(paths).not.toContain("/watchlist");
    expect(paths).not.toContain("/contracts/new");
  });

  it("emits a page per tracked contract with a real timestamp", async () => {
    vi.stubEnv("NEXT_PUBLIC_APP_URL", ORIGIN);
    stubContracts([
      {
        id: "CABC",
        last_activity_at: "2026-11-01T10:00:00Z",
        added_at: "2026-01-01T00:00:00Z",
      },
      {
        id: "CDEF",
        last_activity_at: null,
        added_at: "2026-02-02T00:00:00Z",
      },
    ]);

    const entries = await sitemap();
    const byUrl = new Map(entries.map((entry) => [entry.url, entry]));

    expect(byUrl.get(`${ORIGIN}/contracts/CABC`)?.lastModified).toEqual(
      new Date("2026-11-01T10:00:00Z")
    );
    // No activity yet, so the date the contract was tracked stands in.
    expect(byUrl.get(`${ORIGIN}/contracts/CDEF`)?.lastModified).toEqual(
      new Date("2026-02-02T00:00:00Z")
    );
  });

  it("does not stamp static routes with a fresh timestamp per call", async () => {
    vi.stubEnv("NEXT_PUBLIC_APP_URL", ORIGIN);
    stubContracts([]);

    const first = await sitemap();
    const second = await sitemap();

    const statics = first.filter((entry) => !entry.url.includes("/contracts/"));
    const stamps = new Set(statics.map((entry) => stampOf(entry)));
    expect(stamps.size).toBe(1);
    expect(stampOf(second[0])).toBe(stampOf(first[0]));
  });

  it("follows next_cursor until the list is exhausted", async () => {
    vi.stubEnv("NEXT_PUBLIC_APP_URL", ORIGIN);
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          contracts: [{ id: "C1", added_at: "2026-01-01T00:00:00Z" }],
          next_cursor: "cursor-2",
        }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          contracts: [{ id: "C2", added_at: "2026-01-02T00:00:00Z" }],
          next_cursor: "",
        }),
      });
    vi.stubGlobal("fetch", fetchMock);

    const urls = (await sitemap())
      .map((entry) => entry.url)
      .filter((url) => url.includes("/contracts/"));

    expect(urls).toEqual([`${ORIGIN}/contracts/C1`, `${ORIGIN}/contracts/C2`]);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(String(fetchMock.mock.calls[1][0])).toContain("cursor=cursor-2");
  });

  it("publishes the same origin as robots.txt", async () => {
    vi.stubEnv("NEXT_PUBLIC_APP_URL", `${ORIGIN}/`);
    stubContracts([]);
    const { default: robots } = await import("@/app/robots");

    const entries = await sitemap();
    const robotSitemap = robots().sitemap;

    expect(robotSitemap).toBe(`${ORIGIN}/sitemap.xml`);
    // The `host` and every sitemap URL must name one and the same origin.
    expect(robots().host).toBe(ORIGIN);
    expect(new URL(entries[0].url).origin).toBe(ORIGIN);
  });

  it("still publishes the static routes when the API is unavailable", async () => {
    vi.stubEnv("NEXT_PUBLIC_APP_URL", ORIGIN);
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => ({ ok: false, status: 500, json: async () => ({}) }))
    );

    const entries = await sitemap();

    expect(entries.map((entry) => entry.url)).toContain(`${ORIGIN}/contracts`);
  });
});
