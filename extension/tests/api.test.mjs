import test from "node:test";
import assert from "node:assert/strict";

import {
  SorolensApiError,
  SorolensClient,
  describeError,
} from "../src/shared/api.js";
import {
  API_BASE,
  CONTRACT_ID,
  installFetch,
  jsonResponse,
} from "./fixtures.mjs";

const WATCHLIST_STATUS_PATH = `/api/v1/watchlist/${CONTRACT_ID}/status`;

function client(overrides = {}) {
  return new SorolensClient({
    apiBaseUrl: API_BASE,
    apiKey: "test-key",
    userId: "user_1",
    network: "testnet",
    ...overrides,
  });
}

test("buildUrl joins the base URL, path and query params", () => {
  const api = client();
  assert.equal(
    api.buildUrl("/api/v1/contracts", { limit: 10, network: "testnet" }),
    `${API_BASE}/api/v1/contracts?limit=10&network=testnet`,
  );
  // Empty/undefined params are dropped.
  assert.equal(
    api.buildUrl("/api/v1/contracts", { cursor: "", network: undefined }),
    `${API_BASE}/api/v1/contracts`,
  );
});

test("buildHeaders sends the scoped key and the identity header", () => {
  const headers = client().buildHeaders(true);
  assert.equal(headers.Authorization, "Bearer test-key");
  assert.equal(headers["X-User-ID"], "user_1");
  assert.equal(headers["Content-Type"], "application/json");
  assert.equal(headers.Accept, "application/json");
});

test("buildHeaders omits credentials that are not configured", () => {
  const headers = client({ apiKey: "", userId: "" }).buildHeaders(false);
  assert.equal(headers.Authorization, undefined);
  assert.equal(headers["X-User-ID"], undefined);
  assert.equal(headers["Content-Type"], undefined);
});

test("getContract returns null on 404 instead of throwing", async () => {
  const fetchStub = installFetch(() => jsonResponse(404, { error: "not found" }));
  try {
    assert.equal(await client().getContract(CONTRACT_ID), null);
  } finally {
    fetchStub.restore();
  }
});

test("getTrackingStatus combines contract and watchlist results", async () => {
  const fetchStub = installFetch((request) => {
    if (request.url.pathname === `/api/v1/contracts/${CONTRACT_ID}`) {
      return jsonResponse(200, { id: CONTRACT_ID, status: "active" });
    }
    if (request.url.pathname === WATCHLIST_STATUS_PATH) {
      return jsonResponse(200, { in_watchlist: true });
    }
    return jsonResponse(500, { error: "unexpected path" });
  });
  try {
    const status = await client().getTrackingStatus(CONTRACT_ID);
    assert.deepEqual(status, {
      contractId: CONTRACT_ID,
      tracked: true,
      inWatchlist: true,
      status: "active",
    });
  } finally {
    fetchStub.restore();
  }
});

test("getTrackingStatus reports partial failures without rejecting", async () => {
  const fetchStub = installFetch((request) => {
    if (request.url.pathname === `/api/v1/contracts/${CONTRACT_ID}`) {
      return jsonResponse(200, { id: CONTRACT_ID, status: "active" });
    }
    return jsonResponse(401, { error: "authentication required" });
  });
  try {
    const status = await client().getTrackingStatus(CONTRACT_ID);
    assert.equal(status.tracked, true);
    assert.equal(status.inWatchlist, false);
    assert.match(status.error, /API key missing, invalid or revoked/);
  } finally {
    fetchStub.restore();
  }
});

test("track registers an untracked contract and adds it to the watchlist", async () => {
  const fetchStub = installFetch((request) => {
    const { method, url } = request;
    if (method === "GET" && url.pathname === `/api/v1/contracts/${CONTRACT_ID}`) {
      return jsonResponse(404, { error: "not found" });
    }
    if (method === "POST" && url.pathname === "/api/v1/contracts") {
      assert.deepEqual(request.body, { id: CONTRACT_ID, network: "testnet" });
      return jsonResponse(201, { id: CONTRACT_ID, status: "pending" });
    }
    if (method === "POST" && url.pathname === "/api/v1/watchlist") {
      assert.deepEqual(request.body, { contract_id: CONTRACT_ID });
      return jsonResponse(201, { in_watchlist: true });
    }
    return jsonResponse(500, { error: `unhandled ${method} ${url.pathname}` });
  });
  try {
    const outcome = await client().track(CONTRACT_ID);
    assert.deepEqual(outcome, {
      contractId: CONTRACT_ID,
      registered: true,
      inWatchlist: true,
      warnings: [],
    });
  } finally {
    fetchStub.restore();
  }
});

test("track keeps the watchlist add when registration is forbidden", async () => {
  const fetchStub = installFetch((request) => {
    const { method, url } = request;
    if (method === "GET" && url.pathname === `/api/v1/contracts/${CONTRACT_ID}`) {
      return jsonResponse(404, { error: "not found" });
    }
    if (method === "POST" && url.pathname === "/api/v1/contracts") {
      return jsonResponse(403, { error: "missing scope", required: "write:contracts" });
    }
    if (method === "POST" && url.pathname === "/api/v1/watchlist") {
      return jsonResponse(201, { in_watchlist: true });
    }
    return jsonResponse(500, { error: "unhandled" });
  });
  try {
    const outcome = await client().track(CONTRACT_ID);
    assert.equal(outcome.registered, false);
    assert.equal(outcome.inWatchlist, true);
    assert.equal(outcome.warnings.length, 1);
    assert.match(outcome.warnings[0], /write:contracts/);
  } finally {
    fetchStub.restore();
  }
});

test("untrack calls DELETE and returns the new membership", async () => {
  const fetchStub = installFetch((request) => {
    if (request.method !== "DELETE") return jsonResponse(500, { error: "bad method" });
    return jsonResponse(200, { in_watchlist: false });
  });
  try {
    const result = await client().untrack(CONTRACT_ID);
    assert.deepEqual(result, { contractId: CONTRACT_ID, inWatchlist: false });
    assert.equal(
      fetchStub.calls[0].url.pathname,
      `/api/v1/watchlist/${CONTRACT_ID}`,
    );
  } finally {
    fetchStub.restore();
  }
});

test("request aborts and reports a timeout", async () => {
  const previous = globalThis.fetch;
  globalThis.fetch = (_url, options = {}) =>
    new Promise((_resolve, reject) => {
      options.signal.addEventListener("abort", () => {
        const error = new Error("aborted");
        error.name = "AbortError";
        reject(error);
      });
    });
  try {
    const api = client({ timeoutMs: 5 });
    await assert.rejects(
      () => api.health(),
      (error) => error instanceof SorolensApiError && error.code === "timeout",
    );
  } finally {
    globalThis.fetch = previous;
  }
});

test("listWatchlist maps items to contract ids", async () => {
  const fetchStub = installFetch(() =>
    jsonResponse(200, {
      items: [{ contract_id: CONTRACT_ID, added_at: "2026-01-01T00:00:00Z" }],
    }),
  );
  try {
    assert.deepEqual(await client().listWatchlist(), [CONTRACT_ID]);
  } finally {
    fetchStub.restore();
  }
});

test("describeError explains the common HTTP failures", () => {
  assert.equal(
    describeError(new SorolensApiError("nope", { status: 401 })),
    "API key missing, invalid or revoked",
  );
  assert.match(
    describeError(new SorolensApiError("missing scope", { status: 403 })),
    /required scope or role not granted/,
  );
  assert.equal(
    describeError(new SorolensApiError("slow down", { status: 429 })),
    "Rate limited by the Sorolens API",
  );
  assert.equal(describeError(new Error("boom")), "boom");
  assert.equal(describeError("boom"), "boom");
});
