/**
 * push-routes.test.ts — unit tests for the Web Push API routes.
 *
 * Tests POST/DELETE /api/push/subscribe and GET /api/push/vapid-public-key.
 * Routes are imported directly (not via fetch) so this works without a running
 * Next.js server.
 *
 * Note: web-push is mocked out since we don't want to send real pushes in tests.
 */

import { afterEach, describe, it, expect, vi, beforeEach } from "vitest";
import webPush from "web-push";

// ------------------------------------------------------------------
// Mock web-push so sendNotification never makes network calls
// ------------------------------------------------------------------
vi.mock("web-push", () => ({
  default: {
    setVapidDetails: vi.fn(),
    sendNotification: vi.fn().mockResolvedValue(undefined),
  },
}));

// ------------------------------------------------------------------
// Mock env vars for VAPID
// ------------------------------------------------------------------
beforeEach(() => {
  vi.clearAllMocks();
  vi.stubEnv("PUSH_INTERNAL_SECRET", undefined);
  process.env.VAPID_PUBLIC_KEY =
    "BEl62iUYgUivxIkv69yViEuiBIa-Ib9-SkvMeAtA3LFgDzkrxZJjSgSnfckjBJuBkr3qBUYIHBQFLXYp5Nksh8U";
  process.env.VAPID_PRIVATE_KEY = "q-LAJMHFUsJ5CjJf4KGS3m5c7ANoKRjn0gOj1N_kla0";
  process.env.VAPID_SUBJECT = "mailto:test@sorolens.xyz";
  vi.resetModules();
});

// ------------------------------------------------------------------
// Helpers to build requests
// ------------------------------------------------------------------
function makeRequest(
  method: string,
  body?: unknown,
  headers?: Record<string, string>
): Request {
  return new Request("http://localhost/api/push/subscribe", {
    method,
    headers: { "Content-Type": "application/json", ...headers },
    body: body != null ? JSON.stringify(body) : undefined,
  });
}

afterEach(() => vi.unstubAllEnvs());

const subscription = {
  endpoint: "https://fcm.googleapis.com/test-endpoint",
  keys: {
    auth: Buffer.alloc(16, 1).toString("base64url"),
    p256dh: Buffer.concat([Buffer.of(4), Buffer.alloc(64, 1)]).toString(
      "base64url"
    ),
  },
};

describe("GET /api/push/vapid-public-key", () => {
  it("returns the public VAPID key when env vars are set", async () => {
    const { GET } = await import("@/app/api/push/vapid-public-key/route");
    const res = await GET();
    const data = await res.json();
    expect(res.status).toBe(200);
    expect(data.publicKey).toBe(process.env.VAPID_PUBLIC_KEY);
  });

  it("returns 503 when VAPID keys are not configured", async () => {
    delete process.env.VAPID_PUBLIC_KEY;
    delete process.env.VAPID_PRIVATE_KEY;
    vi.resetModules();

    const { GET } = await import("@/app/api/push/vapid-public-key/route");
    const res = await GET();
    expect(res.status).toBe(503);
    const data = await res.json();
    expect(data.error).toBeTruthy();
  });
});

describe("POST /api/push/subscribe", () => {
  it("accepts a valid subscription payload and returns 201", async () => {
    const { POST } = await import("@/app/api/push/subscribe/route");

    const res = await POST(makeRequest("POST", subscription));
    expect(res.status).toBe(201);
    const data = await res.json();
    expect(data.ok).toBe(true);
  });

  it("returns 400 for missing endpoint", async () => {
    const { POST } = await import("@/app/api/push/subscribe/route");
    const req = makeRequest("POST", { keys: { auth: "a", p256dh: "b" } });
    const res = await POST(req);
    expect(res.status).toBe(400);
  });

  it("returns 400 for missing keys", async () => {
    const { POST } = await import("@/app/api/push/subscribe/route");
    const req = makeRequest("POST", {
      endpoint: "https://fcm.googleapis.com/x",
    });
    const res = await POST(req);
    expect(res.status).toBe(400);
  });
});

describe("DELETE /api/push/subscribe", () => {
  it("accepts a delete request with an endpoint", async () => {
    const { DELETE } = await import("@/app/api/push/subscribe/route");
    const req = makeRequest("DELETE", {
      endpoint: "https://fcm.googleapis.com/test-endpoint",
    });
    const res = await DELETE(req);
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(data.ok).toBe(true);
  });

  it("returns 400 when endpoint is missing", async () => {
    const { DELETE } = await import("@/app/api/push/subscribe/route");
    const req = makeRequest("DELETE", {});
    const res = await DELETE(req);
    expect(res.status).toBe(400);
  });
});

describe("push subscription validation and limits", () => {
  it.each([
    { ...subscription, endpoint: "https://example.com/push" },
    { ...subscription, endpoint: "http://fcm.googleapis.com/push" },
    { ...subscription, keys: { ...subscription.keys, auth: 123 } },
    { ...subscription, keys: { ...subscription.keys, p256dh: "short" } },
    { ...subscription, extra: "x".repeat(8192) },
  ])("rejects an invalid subscription", async (payload) => {
    const { POST } = await import("@/app/api/push/subscribe/route");
    expect((await POST(makeRequest("POST", payload))).status).toBe(400);
  });

  it("limits subscription mutations and returns Retry-After", async () => {
    const { POST, DELETE } = await import("@/app/api/push/subscribe/route");
    for (let i = 0; i < 60; i++) {
      expect((await POST(makeRequest("POST", subscription))).status).toBe(201);
    }
    const response = await DELETE(makeRequest("DELETE", subscription));
    expect(response.status).toBe(429);
    expect(Number(response.headers.get("Retry-After"))).toBeGreaterThan(0);
  });
});

describe("PUT /api/push/subscribe", () => {
  const secret = "test-dispatch-key";

  it.each([
    { configured: undefined, supplied: secret, status: 401 },
    { configured: "", supplied: secret, status: 401 },
    { configured: secret, supplied: "test-dispatch-kez", status: 401 },
    { configured: secret, supplied: "wrong", status: 401 },
    { configured: secret, supplied: undefined, status: 401 },
    { configured: secret, supplied: secret, status: 200 },
  ])(
    "checks the dispatch secret (%j)",
    async ({ configured, supplied, status }) => {
      vi.stubEnv("PUSH_INTERNAL_SECRET", configured);
      const { POST, PUT } = await import("@/app/api/push/subscribe/route");
      expect((await POST(makeRequest("POST", subscription))).status).toBe(201);
      const response = await PUT(
        makeRequest(
          "PUT",
          { title: "Alert" },
          supplied ? { "x-push-secret": supplied } : {}
        )
      );
      expect(response.status).toBe(status);
      if (status === 200)
        expect(await response.json()).toEqual({ sent: 1, failed: 0 });
      expect(webPush.sendNotification).toHaveBeenCalledTimes(
        status === 200 ? 1 : 0
      );
    }
  );

  it("rejects invalid notification types and limits authorized sends", async () => {
    vi.stubEnv("PUSH_INTERNAL_SECRET", secret);
    const { POST, PUT } = await import("@/app/api/push/subscribe/route");
    await POST(makeRequest("POST", subscription));
    const headers = { "x-push-secret": secret };
    const invalid = await PUT(makeRequest("PUT", { title: 123 }, headers));
    expect(invalid.status).toBe(400);
    expect(webPush.sendNotification).not.toHaveBeenCalled();
    for (let i = 0; i < 5; i++) {
      expect((await PUT(makeRequest("PUT", {}, headers))).status).toBe(200);
    }
    expect((await PUT(makeRequest("PUT", {}, headers))).status).toBe(429);
    expect(webPush.sendNotification).toHaveBeenCalledTimes(5);
  });
});
