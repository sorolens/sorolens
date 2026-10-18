/**
 * push-routes.test.ts — unit tests for the Web Push API routes (issue #592).
 *
 * The subscribe route is now a thin proxy onto the persistent Go API store
 * (POST/GET/DELETE /api/v1/push/subscriptions); the vapid-public-key route is
 * unchanged. Upstream calls are exercised through a mocked global.fetch, and
 * web-push itself is mocked so no real push is ever sent.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// ------------------------------------------------------------------
// Mock web-push so sendNotification never makes network calls
// ------------------------------------------------------------------
const sendNotification = vi.fn();
vi.mock("web-push", () => ({
  default: {
    setVapidDetails: vi.fn(),
    sendNotification,
  },
}));

vi.mock("@/lib/config", () => ({ API_URL: "http://api.test" }));

// ------------------------------------------------------------------
// Env vars for VAPID + server-to-API credential
// ------------------------------------------------------------------
const UPSTREAM_KEY = "sl_upstream_key_for_tests";

beforeEach(() => {
  process.env.VAPID_PUBLIC_KEY =
    "BEl62iUYgUivxIkv69yViEuiBIa-Ib9-SkvMeAtA3LFgDzkrxZJjSgSnfckjBJuBkr3qBUYIHBQFLXYp5Nksh8U";
  process.env.VAPID_PRIVATE_KEY = "q-LAJMHFUsJ5CjJf4KGS3m5c7ANoKRjn0gOj1N_kla0";
  process.env.VAPID_SUBJECT = "mailto:test@sorolens.dev";
  process.env.SOROLENS_API_KEY = UPSTREAM_KEY;
  vi.resetModules();
  sendNotification.mockReset();
});

afterEach(() => {
  vi.unstubAllGlobals();
  delete process.env.SOROLENS_API_KEY;
  delete process.env.PUSH_INTERNAL_SECRET;
});

// ------------------------------------------------------------------
// Helpers
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

function jsonResponse(status: number, data: unknown): Response {
  return new Response(JSON.stringify(data), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const validSub = {
  endpoint: "https://fcm.googleapis.com/fcm/send/abc123",
  keys: {
    auth: "AAAAAAAAAAAAAAAAAAAAAA",
    p256dh:
      "BEl62iUYgUivxIkv69yViEuiBIa-Ib9-SkvMeAtA3LFgDzkrxZJjSgSnfckjBJuBkr3qBUYIHBQFLXYp5Nksh8U",
  },
};

async function loadSubscribeRoute() {
  return await import("@/app/api/push/subscribe/route");
}

// ------------------------------------------------------------------
// GET /api/push/vapid-public-key (unchanged behaviour)
// ------------------------------------------------------------------
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

// ------------------------------------------------------------------
// POST /api/push/subscribe — proxy to the persistent API store
// ------------------------------------------------------------------
describe("POST /api/push/subscribe (API proxy)", () => {
  it("proxies a valid subscription and passes through 201", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse(201, { id: "sub-1", endpoint: validSub.endpoint }));
    vi.stubGlobal("fetch", fetchMock);

    const { POST } = await loadSubscribeRoute();
    const res = await POST(
      makeRequest("POST", validSub) as unknown as Parameters<typeof POST>[0]
    );
    expect(res.status).toBe(201);
    expect(await res.json()).toMatchObject({ ok: true, id: "sub-1" });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("http://api.test/api/v1/push/subscriptions");
    expect(init.method).toBe("POST");
    expect(init.headers.Authorization).toBe(`Bearer ${UPSTREAM_KEY}`);
    expect(JSON.parse(init.body).endpoint).toBe(validSub.endpoint);
  });

  it("passes through 200 for a duplicate endpoint (API idempotency)", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(jsonResponse(200, { id: "sub-1" }))
    );
    const { POST } = await loadSubscribeRoute();
    const res = await POST(
      makeRequest("POST", validSub) as unknown as Parameters<typeof POST>[0]
    );
    expect(res.status).toBe(200);
    expect((await res.json()).ok).toBe(true);
  });

  it("rejects malformed payloads locally without calling the API", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    const { POST } = await loadSubscribeRoute();

    const bad = [
      { keys: { auth: "a", p256dh: "b" } }, // no endpoint
      { endpoint: "https://x.example/1" }, // no keys
      { endpoint: "http://insecure.example/1", keys: { auth: "a", p256dh: "b" } }, // not https
      { endpoint: "https://x.example/1", keys: { auth: "", p256dh: "b" } }, // empty auth
    ];
    for (const body of bad) {
      const res = await POST(
        makeRequest("POST", body) as unknown as Parameters<typeof POST>[0]
      );
      expect(res.status).toBe(400);
    }
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("maps the API's 422 validation error to a 400 with its message", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        jsonResponse(422, {
          error: { code: "INVALID_INPUT", message: "endpoint must be a valid https URL" },
        })
      )
    );
    const { POST } = await loadSubscribeRoute();
    const res = await POST(
      makeRequest("POST", validSub) as unknown as Parameters<typeof POST>[0]
    );
    expect(res.status).toBe(400);
    const data = await res.json();
    expect(data.error).toBe("endpoint must be a valid https URL");
    // The failure body must not leak the upstream bearer key.
    expect(JSON.stringify(data)).not.toContain(UPSTREAM_KEY);
  });

  it("answers 502 when the API is unreachable", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("ECONNREFUSED")));
    const { POST } = await loadSubscribeRoute();
    const res = await POST(
      makeRequest("POST", validSub) as unknown as Parameters<typeof POST>[0]
    );
    expect(res.status).toBe(502);
  });

  it("still works anonymously when SOROLENS_API_KEY is unset", async () => {
    delete process.env.SOROLENS_API_KEY;
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse(201, { id: "sub-2" }));
    vi.stubGlobal("fetch", fetchMock);
    const { POST } = await loadSubscribeRoute();
    const res = await POST(
      makeRequest("POST", validSub) as unknown as Parameters<typeof POST>[0]
    );
    expect(res.status).toBe(201);
    expect(fetchMock.mock.calls[0][1].headers.Authorization).toBeUndefined();
  });
});

// ------------------------------------------------------------------
// DELETE /api/push/subscribe
// ------------------------------------------------------------------
describe("DELETE /api/push/subscribe (API proxy)", () => {
  it("unsubscribes by endpoint, mapping the API's 204 to ok:true", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);

    const { DELETE } = await loadSubscribeRoute();
    const res = await DELETE(
      makeRequest("DELETE", { endpoint: validSub.endpoint }) as unknown as Parameters<typeof DELETE>[0]
    );
    expect(res.status).toBe(200);
    expect((await res.json()).ok).toBe(true);
    expect(fetchMock.mock.calls[0][0]).toBe(
      `http://api.test/api/v1/push/subscriptions?endpoint=${encodeURIComponent(validSub.endpoint)}`
    );
    expect(fetchMock.mock.calls[0][1].method).toBe("DELETE");
  });

  it("surfaces a 404 from the store", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(jsonResponse(404, { error: { message: "subscription not found" } }))
    );
    const { DELETE } = await loadSubscribeRoute();
    const res = await DELETE(
      makeRequest("DELETE", { endpoint: validSub.endpoint }) as unknown as Parameters<typeof DELETE>[0]
    );
    expect(res.status).toBe(404);
  });

  it("returns 400 when endpoint is missing", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    const { DELETE } = await loadSubscribeRoute();
    const res = await DELETE(
      makeRequest("DELETE", {}) as unknown as Parameters<typeof DELETE>[0]
    );
    expect(res.status).toBe(400);
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

// ------------------------------------------------------------------
// PUT (internal fan-out) — reads subscriptions from the API
// ------------------------------------------------------------------
describe("PUT /api/push/subscribe (internal fan-out)", () => {
  const stored = {
    subscriptions: [
      {
        endpoint: validSub.endpoint,
        p256dh_key: validSub.keys.p256dh,
        auth_key: validSub.keys.auth,
      },
    ],
  };

  it("sends to every subscription read from the persistent store", async () => {
    sendNotification.mockResolvedValue(undefined);
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse(200, stored));
    vi.stubGlobal("fetch", fetchMock);

    const { PUT } = await loadSubscribeRoute();
    const res = await PUT(
      makeRequest("PUT", { title: "Alert", body: "contract degraded" }) as unknown as Parameters<typeof PUT>[0]
    );
    expect(res.status).toBe(200);
    expect(await res.json()).toEqual({ sent: 1, failed: 0 });
    expect(fetchMock.mock.calls[0][0]).toBe(
      "http://api.test/api/v1/push/subscriptions"
    );
  });

  it("purges subscriptions the push service reports as gone (410)", async () => {
    sendNotification.mockRejectedValue({ statusCode: 410 });
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse(200, stored))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);

    const { PUT } = await loadSubscribeRoute();
    const res = await PUT(
      makeRequest("PUT", { title: "Alert" }) as unknown as Parameters<typeof PUT>[0]
    );
    expect(await res.json()).toEqual({ sent: 0, failed: 1 });
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(String(fetchMock.mock.calls[1][0])).toContain(
      "subscriptions?endpoint="
    );
    expect(fetchMock.mock.calls[1][1].method).toBe("DELETE");
  });

  it("requires SOROLENS_API_KEY to reach the store", async () => {
    delete process.env.SOROLENS_API_KEY;
    const { PUT } = await loadSubscribeRoute();
    const res = await PUT(
      makeRequest("PUT", { title: "Alert" }) as unknown as Parameters<typeof PUT>[0]
    );
    expect(res.status).toBe(503);
  });

  it("keeps the internal shared-secret guard", async () => {
    process.env.PUSH_INTERNAL_SECRET = "ses3cret";
    const { PUT } = await loadSubscribeRoute();
    const res = await PUT(
      new Request("http://localhost/api/push/subscribe", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ title: "x" }),
      }) as unknown as Parameters<typeof PUT>[0]
    );
    expect(res.status).toBe(401);
  });
});
