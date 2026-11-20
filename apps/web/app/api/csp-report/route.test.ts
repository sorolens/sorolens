// @vitest-environment node
import { NextRequest } from "next/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const validReport = JSON.stringify({
  "csp-report": {
    "document-uri": "https://sorolens.xyz/contracts?secret=1",
    "effective-directive": "script-src-elem",
    "blocked-uri": "inline",
  },
});

function post(
  body: string,
  headers: Record<string, string> = {
    "content-type": "application/csp-report",
  }
) {
  return new NextRequest("http://localhost/api/csp-report", {
    method: "POST",
    body,
    headers,
  });
}

// The limiter lives at module scope, so each test gets a fresh module.
async function loadRoute() {
  vi.resetModules();
  return import("./route");
}

let warn: ReturnType<typeof vi.spyOn>;
beforeEach(() => {
  warn = vi.spyOn(console, "warn").mockImplementation(() => {});
});
afterEach(() => warn.mockRestore());

describe("POST /api/csp-report", () => {
  it("accepts a valid report and logs only the sanitised summary", async () => {
    const { POST } = await loadRoute();
    const res = await POST(post(validReport));

    expect(res.status).toBe(204);
    expect(warn).toHaveBeenCalledTimes(1);
    const logged = warn.mock.calls[0].join(" ");
    expect(logged).toContain("script-src-elem");
    expect(logged).toContain("/contracts");
    expect(logged).not.toContain("secret");
  });

  it("accepts a Reporting API batch", async () => {
    const { POST } = await loadRoute();
    const res = await POST(
      post(
        JSON.stringify([
          {
            type: "csp-violation",
            body: {
              documentURL: "https://sorolens.xyz/",
              effectiveDirective: "img-src",
              blockedURL: "https://x.example/a.png",
            },
          },
        ]),
        { "content-type": "application/reports+json" }
      )
    );
    expect(res.status).toBe(204);
  });

  it("rejects an unsupported content type", async () => {
    const { POST } = await loadRoute();
    const res = await POST(post(validReport, { "content-type": "text/plain" }));
    expect(res.status).toBe(415);
  });

  it("rejects malformed JSON and invalid reports without logging", async () => {
    const { POST } = await loadRoute();
    expect((await POST(post("{nope"))).status).toBe(400);
    expect((await POST(post(JSON.stringify({ a: 1 })))).status).toBe(400);
    expect(warn).not.toHaveBeenCalled();
  });

  it("rejects an oversized body", async () => {
    const { POST } = await loadRoute();
    const res = await POST(post("x".repeat(17 * 1024)));
    expect(res.status).toBe(413);
  });

  it("rate limits a noisy client without affecting others", async () => {
    const { POST } = await loadRoute();
    const from = (ip: string) =>
      post(validReport, {
        "content-type": "application/csp-report",
        "x-forwarded-for": ip,
      });

    for (let i = 0; i < 30; i++) {
      expect((await POST(from("203.0.113.1"))).status).toBe(204);
    }
    expect((await POST(from("203.0.113.1"))).status).toBe(429);
    expect((await POST(from("203.0.113.2"))).status).toBe(204);
  });
});
