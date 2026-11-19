import { describe, expect, it } from "vitest";
import { buildCsp, cspHeaderName, generateNonce } from "./csp";

const base = {
  nonce: "abc123",
  isDev: false,
  apiUrl: "https://api.sorolens.dev/base",
};

function directive(policy: string, name: string): string[] {
  const match = policy
    .split("; ")
    .find((entry) => entry.split(" ")[0] === name);
  return match ? match.split(" ").slice(1) : [];
}

describe("generateNonce", () => {
  it("returns a distinct base64 value of 128 bits each call", () => {
    const first = generateNonce();
    expect(first).toMatch(/^[A-Za-z0-9+/]{22}==$/);
    expect(generateNonce()).not.toBe(first);
  });
});

describe("buildCsp", () => {
  it("allows scripts only through the nonce in production", () => {
    const scripts = directive(buildCsp(base), "script-src");
    expect(scripts).toEqual(["'self'", "'nonce-abc123'", "'strict-dynamic'"]);
  });

  it("never allows inline scripts or eval in production", () => {
    const policy = buildCsp({
      ...base,
      sentryDsn: "https://key@o0.ingest.sentry.io/1",
      plausibleDomain: "sorolens.dev",
    });
    expect(directive(policy, "script-src")).not.toContain("'unsafe-inline'");
    expect(policy).not.toContain("'unsafe-eval'");
    expect(directive(policy, "style-src")).not.toContain("'unsafe-inline'");
  });

  it("adds eval and the hot-reload socket only in development", () => {
    const policy = buildCsp({ ...base, isDev: true });
    expect(directive(policy, "script-src")).toContain("'unsafe-eval'");
    expect(directive(policy, "connect-src")).toContain("ws://localhost:*");
  });

  it("locks down plugins, framing, base and form targets", () => {
    const policy = buildCsp(base);
    expect(directive(policy, "object-src")).toEqual(["'none'"]);
    expect(directive(policy, "frame-ancestors")).toEqual(["'none'"]);
    expect(directive(policy, "base-uri")).toEqual(["'self'"]);
    expect(directive(policy, "form-action")).toEqual(["'self'"]);
  });

  it("permits the API origin for REST, sockets and badges", () => {
    const policy = buildCsp(base);
    const connect = directive(policy, "connect-src");
    expect(connect).toContain("https://api.sorolens.dev");
    expect(connect).toContain("wss://api.sorolens.dev");
    expect(directive(policy, "img-src")).toContain("https://api.sorolens.dev");
  });

  it("only opens analytics and error reporting when configured", () => {
    expect(buildCsp(base)).not.toContain("plausible.io");
    expect(buildCsp(base)).not.toContain("sentry.io");

    const connect = directive(
      buildCsp({
        ...base,
        sentryDsn: "https://key@o0.ingest.sentry.io/1",
        plausibleDomain: "sorolens.dev",
      }),
      "connect-src"
    );
    expect(connect).toContain("https://o0.ingest.sentry.io");
    expect(connect).toContain("https://plausible.io");
  });

  it("ignores a malformed API URL", () => {
    const policy = buildCsp({ ...base, apiUrl: "not a url" });
    expect(directive(policy, "connect-src")).toEqual(["'self'"]);
  });

  it("names the report endpoint", () => {
    const policy = buildCsp(base);
    expect(directive(policy, "report-uri")).toEqual(["/api/csp-report"]);
    expect(directive(policy, "report-to")).toEqual(["csp-endpoint"]);
  });
});

describe("cspHeaderName", () => {
  it("enforces by default", () => {
    expect(cspHeaderName(undefined)).toBe("Content-Security-Policy");
    expect(cspHeaderName("false")).toBe("Content-Security-Policy");
  });

  it("switches to report-only when enabled", () => {
    expect(cspHeaderName("true")).toBe("Content-Security-Policy-Report-Only");
  });
});
