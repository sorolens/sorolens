import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("./user", () => ({
  ensureSession: vi.fn(async () => ({
    user_id: "sess_test",
    token: "sls_session_token",
    expires_at: "2099-01-01T00:00:00.000Z",
  })),
}));

import { ApiError } from "./api";
import { createMyApiKey, listMyApiKeys, revokeMyApiKey } from "./apiKeys";

function jsonResponse(body: unknown, status = 200): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText: status === 200 ? "OK" : "Error",
    json: async () => body,
  } as Response;
}

const KEY = {
  id: "key-1",
  name: "Dashboard key",
  key_prefix: "sl_abcdefgh",
  scopes: ["read:contracts"],
  user_id: "sess_test",
  created_at: "2026-10-01T00:00:00.000Z",
  last_used_at: null,
  revoked_at: null,
  rotated_at: null,
};

describe("apiKeys client", () => {
  const fetchMock = vi.fn();

  beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("lists keys with the session bearer token", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ api_keys: [KEY] }));

    await expect(listMyApiKeys()).resolves.toEqual([KEY]);

    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toContain("/api/v1/me/api-keys");
    expect(init.headers.Authorization).toBe("Bearer sls_session_token");
  });

  it("creates a key through the API and returns the one-time plaintext", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({ ...KEY, key: "sl_plaintext" }, 201)
    );

    const created = await createMyApiKey("ci", ["read:watchdog"]);
    expect(created.key).toBe("sl_plaintext");

    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toContain("/api/v1/me/api-keys");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toEqual({
      name: "ci",
      scopes: ["read:watchdog"],
    });
  });

  it("revokes a key by id", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(null, 204));

    await revokeMyApiKey("key-1");

    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toContain("/api/v1/me/api-keys/key-1");
    expect(init.method).toBe("DELETE");
  });

  it("surfaces the server's error text", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({ error: "authentication required" }, 401)
    );

    const error = await listMyApiKeys().catch((err: unknown) => err);
    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({
      status: 401,
      message: "authentication required",
    });
  });
});
