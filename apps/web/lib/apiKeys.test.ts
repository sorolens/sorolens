/**
 * Tests for apps/web/lib/apiKeys.ts (issue #613): key creation and revocation
 * go to the real API endpoints, carry the session bearer header, and surface
 * the server's own error messages instead of fabricating a local token.
 */

import { afterEach, describe, expect, it, vi } from "vitest";

import { createApiKey, revokeApiKey } from "./apiKeys";
import { SESSION_STORAGE_KEY } from "./user";

const FUTURE = new Date(Date.now() + 3600_000).toISOString();

function seedSession(userId = "u_42") {
  localStorage.setItem(
    SESSION_STORAGE_KEY,
    JSON.stringify({ user_id: userId, token: "sls_sig", expires_at: FUTURE })
  );
}

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});

describe("createApiKey", () => {
  it("posts to the API and returns the created key with its one-time token", async () => {
    seedSession();
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse(201, {
        id: "key_1",
        name: "Sorolens settings key",
        key_prefix: "sl_ab12cd3",
        created_at: "2026-10-01T00:00:00Z",
        key: "sl_ab12cd3_full_token",
      })
    );
    vi.stubGlobal("fetch", fetchMock);

    const created = await createApiKey({
      name: "Sorolens settings key",
      scopes: ["read:contracts"],
      userId: "u_42",
    });

    expect(created.id).toBe("key_1");
    expect(created.key).toBe("sl_ab12cd3_full_token");
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toContain("/api/v1/api-keys");
    expect(init.method).toBe("POST");
    expect(init.headers.Authorization).toBe("Bearer sls_sig");
    expect(JSON.parse(init.body)).toEqual({
      name: "Sorolens settings key",
      scopes: ["read:contracts"],
    });
  });

  it("surfaces the API's own message when the caller lacks admin scope", async () => {
    seedSession();
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          jsonResponse(403, { error: "admin role required", code: "forbidden" })
        )
    );

    await expect(
      createApiKey({ name: "k", scopes: ["read:contracts"], userId: "u_42" })
    ).rejects.toMatchObject({ status: 403, message: "admin role required" });
  });

  it("rejects incomplete success payloads instead of inventing a key", async () => {
    seedSession();
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          jsonResponse(201, { id: "key_1", name: "k", key_prefix: "p" })
        )
    );

    await expect(
      createApiKey({ name: "k", scopes: ["read:contracts"], userId: "u_42" })
    ).rejects.toThrow(/incomplete/);
  });
});

describe("revokeApiKey", () => {
  it("deletes by id and accepts the 204 no-content response", async () => {
    seedSession();
    const fetchMock = vi
      .fn()
      .mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(revokeApiKey("key_1", "u_42")).resolves.toBeUndefined();
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toContain("/api/v1/api-keys/key_1");
    expect(init.method).toBe("DELETE");
    expect(init.headers.Authorization).toBe("Bearer sls_sig");
  });

  it("surfaces the API error when revocation fails", async () => {
    seedSession();
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(jsonResponse(404, { error: "API key not found" }))
    );

    await expect(revokeApiKey("key_gone", "u_42")).rejects.toMatchObject({
      status: 404,
      message: "API key not found",
    });
  });
});
