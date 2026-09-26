import { describe, it, expect } from "vitest";
import {
  ApiError,
  SorolensApi,
  isValidContractId,
  USER_KEY_SCOPES,
} from "./api.js";

const CONTRACT = "C" + "A".repeat(55);

interface Recorded {
  url: string;
  method: string;
  headers: Record<string, string>;
  body: unknown;
}

/** Build a fake fetch that records requests and replays one JSON response. */
function fakeFetch(payload: unknown, status = 200) {
  const calls: Recorded[] = [];
  const impl = (async (url: string | URL, init: RequestInit = {}) => {
    const headers: Record<string, string> = {};
    for (const [k, v] of Object.entries((init.headers ?? {}) as Record<string, string>)) {
      headers[k.toLowerCase()] = v;
    }
    calls.push({
      url: String(url),
      method: init.method ?? "GET",
      headers,
      body: init.body ? JSON.parse(String(init.body)) : undefined,
    });
    return new Response(responseBody(status, payload), {
      status,
      headers: { "Content-Type": "application/json" },
    });
  }) as unknown as typeof fetch;
  return { impl, calls };
}

/** 204/304 responses must have a null body, so only serialise for real bodies. */
function responseBody(status: number, payload: unknown): string | null {
  if (status === 204 || status === 304) return null;
  return payload === undefined ? "" : JSON.stringify(payload);
}

function apiWith(payload: unknown, status = 200, baseUrl = "https://api.sorolens.test/") {
  const { impl, calls } = fakeFetch(payload, status);
  const api = new SorolensApi({ baseUrl, apiKey: "sl_admin", fetchImpl: impl });
  return { api, calls };
}

describe("isValidContractId", () => {
  it("accepts a 56-char strkey starting with C", () => {
    expect(isValidContractId(CONTRACT)).toBe(true);
  });

  it("accepts a lower-case id by normalising it", () => {
    expect(isValidContractId(CONTRACT.toLowerCase())).toBe(true);
  });

  it("rejects the wrong length, prefix, or alphabet", () => {
    expect(isValidContractId("C" + "A".repeat(54))).toBe(false);
    expect(isValidContractId("G" + "A".repeat(55))).toBe(false);
    expect(isValidContractId("C" + "1".repeat(55))).toBe(false);
  });
});

describe("SorolensApi.getContractStatus", () => {
  it("hits the contract endpoint and normalises the payload", async () => {
    const { api, calls } = apiWith({
      id: CONTRACT,
      network: "testnet",
      label: "Watchdog",
      status: "active",
      wasm_hash: "abc123",
      added_at: "2026-01-01T00:00:00Z",
    });

    const status = await api.getContractStatus(CONTRACT);

    expect(calls[0].url).toBe(`https://api.sorolens.test/api/v1/contracts/${CONTRACT}`);
    expect(calls[0].method).toBe("GET");
    expect(calls[0].headers.authorization).toBe("Bearer sl_admin");
    expect(status).toEqual({
      id: CONTRACT,
      network: "testnet",
      label: "Watchdog",
      status: "active",
      wasmHash: "abc123",
      addedAt: "2026-01-01T00:00:00Z",
    });
  });

  it("prefers the caller's own key and forwards the user id", async () => {
    const { api, calls } = apiWith({ id: CONTRACT });
    await api.getContractStatus(CONTRACT, { apiKey: "sl_user", userId: "discord-42" });
    expect(calls[0].headers.authorization).toBe("Bearer sl_user");
    expect(calls[0].headers["x-user-id"]).toBe("discord-42");
  });

  it("omits Authorization when neither caller nor client has a key", async () => {
    const { impl, calls } = fakeFetch({ id: CONTRACT });
    const api = new SorolensApi({ baseUrl: "https://api.sorolens.test", fetchImpl: impl });
    await api.getContractStatus(CONTRACT);
    expect(calls[0].headers.authorization).toBeUndefined();
  });

  it("defaults missing contract fields instead of throwing", async () => {
    const { api } = apiWith({ id: CONTRACT });
    const status = await api.getContractStatus(CONTRACT);
    expect(status.network).toBe("unknown");
    expect(status.status).toBe("unknown");
    expect(status.label).toBeNull();
  });
});

describe("SorolensApi.listAlerts", () => {
  it("requests the watchdog alerts endpoint and normalises entries", async () => {
    const { api, calls } = apiWith({
      alerts: [
        {
          contract_id: CONTRACT,
          severity: "Critical",
          message: "TTL below threshold",
          ledger: 1234,
          tx_hash: "deadbeef",
          timestamp: "2026-01-02T00:00:00Z",
        },
      ],
    });

    const alerts = await api.listAlerts(CONTRACT, 3);

    expect(calls[0].url).toBe(
      `https://api.sorolens.test/api/v1/watchdog/contracts/${CONTRACT}/alerts?limit=3`,
    );
    expect(alerts[0]).toEqual({
      contractId: CONTRACT,
      severity: "Critical",
      message: "TTL below threshold",
      ledger: 1234,
      txHash: "deadbeef",
      timestamp: "2026-01-02T00:00:00Z",
    });
  });

  it("clamps the limit to the 1-10 range the API accepts", async () => {
    const { api, calls } = apiWith({ alerts: [] });
    await api.listAlerts(CONTRACT, 99);
    expect(calls[0].url).toContain("limit=10");
  });

  it("returns an empty list when the API omits alerts", async () => {
    const { api } = apiWith({});
    expect(await api.listAlerts(CONTRACT)).toEqual([]);
  });
});

describe("SorolensApi watchlist", () => {
  it("adds a contract with the user id header", async () => {
    const { api, calls } = apiWith({ in_watchlist: true });
    const watched = await api.addToWatchlist(CONTRACT, { userId: "discord-7" });
    expect(watched).toBe(true);
    expect(calls[0].method).toBe("POST");
    expect(calls[0].url).toBe("https://api.sorolens.test/api/v1/watchlist");
    expect(calls[0].headers["x-user-id"]).toBe("discord-7");
    expect(calls[0].body).toEqual({ contract_id: CONTRACT });
  });

  it("removes a contract and reports the post-condition", async () => {
    const { api, calls } = apiWith({ in_watchlist: false });
    expect(await api.removeFromWatchlist(CONTRACT, { userId: "d" })).toBe(false);
    expect(calls[0].method).toBe("DELETE");
    expect(calls[0].url).toBe(
      `https://api.sorolens.test/api/v1/watchlist/${CONTRACT}`,
    );
  });

  it("lists watched contract ids, dropping malformed rows", async () => {
    const { api } = apiWith({ items: [{ contract_id: CONTRACT }, {}, { contract_id: "" }] });
    expect(await api.listWatchlist()).toEqual([CONTRACT]);
  });
});

describe("SorolensApi key provisioning", () => {
  it("creates an admin-scoped key and returns its material", async () => {
    const { api, calls } = apiWith({ id: "key_1", key: "sl_new", key_prefix: "sl_abcd1234" });
    const grant = await api.provisionApiKey("discord:1:alice");
    expect(grant).toEqual({ id: "key_1", key: "sl_new", keyPrefix: "sl_abcd1234" });
    expect(calls[0].method).toBe("POST");
    expect(calls[0].url).toBe("https://api.sorolens.test/api/v1/api-keys");
    expect(calls[0].body).toEqual({ name: "discord:1:alice", scopes: USER_KEY_SCOPES });
  });

  it("throws when the API does not return key material", async () => {
    const { api } = apiWith({ id: "key_1" });
    await expect(api.provisionApiKey("n")).rejects.toBeInstanceOf(ApiError);
  });

  it("revokes by id and accepts 204 with no body", async () => {
    const { impl, calls } = fakeFetch(undefined, 204);
    const api = new SorolensApi({ baseUrl: "https://api.sorolens.test", fetchImpl: impl });
    await expect(api.revokeApiKey("key_1")).resolves.toBeUndefined();
    expect(calls[0].url).toBe("https://api.sorolens.test/api/v1/api-keys/key_1");
  });
});

describe("SorolensApi errors", () => {
  it("surfaces the API's status and code", async () => {
    const { api } = apiWith({ error: "contract not found", code: "not_found" }, 404);
    const err = await api.getContractStatus(CONTRACT).catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(404);
    expect((err as ApiError).code).toBe("not_found");
    expect((err as ApiError).message).toBe("contract not found");
  });

  it("reports transport failures as a status-0 ApiError", async () => {
    const impl = (async () => {
      throw new TypeError("fetch failed");
    }) as unknown as typeof fetch;
    const api = new SorolensApi({ baseUrl: "https://api.sorolens.test", fetchImpl: impl });
    const err = await api.getContractStatus(CONTRACT).catch((e) => e);
    expect((err as ApiError).status).toBe(0);
    expect((err as ApiError).message).toContain("could not reach Sorolens API");
  });
});
