/**
 * push-routes.test.ts — unit tests for the Web Push API routes.
 *
 * Tests POST/DELETE /api/push/subscribe and GET /api/push/vapid-public-key.
 * Routes are imported directly (not via fetch) so this works without a running
 * Next.js server.
 *
 * Note: web-push is mocked out since we don't want to send real pushes in tests.
 */

import { describe, it, expect, vi, beforeEach } from "vitest";

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
  process.env.VAPID_PUBLIC_KEY =
    "BEl62iUYgUivxIkv69yViEuiBIa-Ib9-SkvMeAtA3LFgDzkrxZJjSgSnfckjBJuBkr3qBUYIHBQFLXYp5Nksh8U";
  process.env.VAPID_PRIVATE_KEY = "q-LAJMHFUsJ5CjJf4KGS3m5c7ANoKRjn0gOj1N_kla0";
  process.env.VAPID_SUBJECT = "mailto:test@sorolens.dev";
  vi.resetModules();
});

// ------------------------------------------------------------------
// Helpers to build NextRequest-like objects
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

    const body = {
      endpoint: "https://fcm.googleapis.com/test-endpoint",
      keys: {
        auth: "auth-key-base64",
        p256dh: "p256dh-key-base64",
      },
    };

    // NextRequest is not available in vitest; route handlers accept
    // standard Request objects in Next.js 15 App Router
    const req = makeRequest("POST", body);
    const res = await POST(req as unknown as Parameters<typeof POST>[0]);
    expect(res.status).toBe(201);
    const data = await res.json();
    expect(data.ok).toBe(true);
  });

  it("returns 400 for missing endpoint", async () => {
    const { POST } = await import("@/app/api/push/subscribe/route");
    const req = makeRequest("POST", { keys: { auth: "a", p256dh: "b" } });
    const res = await POST(req as unknown as Parameters<typeof POST>[0]);
    expect(res.status).toBe(400);
  });

  it("returns 400 for missing keys", async () => {
    const { POST } = await import("@/app/api/push/subscribe/route");
    const req = makeRequest("POST", {
      endpoint: "https://fcm.googleapis.com/x",
    });
    const res = await POST(req as unknown as Parameters<typeof POST>[0]);
    expect(res.status).toBe(400);
  });
});

describe("DELETE /api/push/subscribe", () => {
  it("accepts a delete request with an endpoint", async () => {
    const { DELETE } = await import("@/app/api/push/subscribe/route");
    const req = makeRequest("DELETE", {
      endpoint: "https://fcm.googleapis.com/test-endpoint",
    });
    const res = await DELETE(req as unknown as Parameters<typeof DELETE>[0]);
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(data.ok).toBe(true);
  });

  it("returns 400 when endpoint is missing", async () => {
    const { DELETE } = await import("@/app/api/push/subscribe/route");
    const req = makeRequest("DELETE", {});
    const res = await DELETE(req as unknown as Parameters<typeof DELETE>[0]);
    expect(res.status).toBe(400);
  });
});
