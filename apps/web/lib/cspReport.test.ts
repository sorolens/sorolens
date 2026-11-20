import { describe, expect, it } from "vitest";
import { createRateLimiter, parseCspReports } from "./cspReport";

const legacy = (overrides: Record<string, unknown> = {}) => ({
  "csp-report": {
    "document-uri": "https://sorolens.dev/contracts/C1?token=secret#frag",
    "effective-directive": "script-src-elem",
    "blocked-uri": "https://evil.example/steal.js?user=alice",
    disposition: "enforce",
    ...overrides,
  },
});

const batch = (body: Record<string, unknown> = {}) => [
  {
    type: "csp-violation",
    body: {
      documentURL: "https://sorolens.dev/settings?email=a@b.c",
      effectiveDirective: "connect-src",
      blockedURL: "wss://evil.example/socket",
      disposition: "report",
      ...body,
    },
  },
];

describe("parseCspReports", () => {
  it("summarises a legacy report without its query strings", () => {
    expect(parseCspReports(legacy())).toEqual([
      {
        directive: "script-src-elem",
        blocked: "https://evil.example",
        documentPath: "/contracts/C1",
        disposition: "enforce",
      },
    ]);
  });

  it("summarises a Reporting API batch", () => {
    expect(parseCspReports(batch())).toEqual([
      {
        directive: "connect-src",
        blocked: "wss://evil.example",
        documentPath: "/settings",
        disposition: "report",
      },
    ]);
  });

  it("keeps keyword sources and reduces data URLs to their scheme", () => {
    expect(
      parseCspReports(legacy({ "blocked-uri": "inline" }))?.[0].blocked
    ).toBe("inline");
    expect(
      parseCspReports(legacy({ "blocked-uri": "data:text/html,secret" }))?.[0]
        .blocked
    ).toBe("data");
  });

  it("drops fields that are never needed", () => {
    const [summary] = parseCspReports(
      legacy({ "script-sample": "alert('secret')", referrer: "https://x.io" })
    )!;
    expect(JSON.stringify(summary)).not.toContain("secret");
    expect(JSON.stringify(summary)).not.toContain("x.io");
  });

  it("falls back to violated-directive and takes its name only", () => {
    const report = legacy({
      "effective-directive": undefined,
      "violated-directive": "img-src 'self'",
    });
    expect(parseCspReports(report)?.[0].directive).toBe("img-src");
  });

  it.each([
    ["null", null],
    ["a string", "report"],
    ["an empty object", {}],
    ["an empty batch", []],
    ["a non-violation entry", [{ type: "deprecation", body: {} }]],
    ["a bad directive", legacy({ "effective-directive": "<script>" })],
    ["a missing document", legacy({ "document-uri": undefined })],
    ["a relative document", legacy({ "document-uri": "/contracts" })],
    ["an oversized field", legacy({ "document-uri": "x".repeat(3000) })],
    ["an oversized batch", Array(21).fill(batch()[0])],
  ])("rejects %s", (_name, payload) => {
    expect(parseCspReports(payload)).toBeNull();
  });
});

describe("createRateLimiter", () => {
  const options = { limit: 2, windowMs: 1000, maxClients: 2 };

  it("limits each client independently", () => {
    const allow = createRateLimiter(options);
    expect(allow("a", 0)).toBe(true);
    expect(allow("a", 1)).toBe(true);
    expect(allow("a", 2)).toBe(false);
    expect(allow("b", 3)).toBe(true);
  });

  it("resets after the window", () => {
    const allow = createRateLimiter(options);
    allow("a", 0);
    allow("a", 1);
    expect(allow("a", 2)).toBe(false);
    expect(allow("a", 1000)).toBe(true);
  });

  it("refuses new clients when full of live windows", () => {
    const allow = createRateLimiter(options);
    allow("a", 0);
    allow("b", 0);
    expect(allow("c", 1)).toBe(false);
  });

  it("evicts expired clients to make room", () => {
    const allow = createRateLimiter(options);
    allow("a", 0);
    allow("b", 0);
    expect(allow("c", 1000)).toBe(true);
  });
});
