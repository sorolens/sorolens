import test from "node:test";
import assert from "node:assert/strict";

import { MESSAGE } from "../src/shared/messages.js";
import { saveSettings } from "../src/shared/storage.js";
import { __test__ } from "../src/background/service-worker.js";
import {
  API_BASE,
  CONTRACT_ID,
  installFakeBrowser,
  installFetch,
  jsonResponse,
} from "./fixtures.mjs";

const { handleMessage } = __test__;

const CONTRACT_PATH = `/api/v1/contracts/${CONTRACT_ID}`;
const STATUS_PATH = `/api/v1/watchlist/${CONTRACT_ID}/status`;
const WATCHLIST_PATH = "/api/v1/watchlist";

/** Canned API used by most cases: contract indexed, not on the watchlist. */
function defaultApi(request) {
  const { method, url } = request;
  if (method === "GET" && url.pathname === CONTRACT_PATH) {
    return jsonResponse(200, { id: CONTRACT_ID, network: "testnet", status: "active" });
  }
  if (method === "GET" && url.pathname === STATUS_PATH) {
    return jsonResponse(200, { in_watchlist: false });
  }
  if (method === "POST" && url.pathname === WATCHLIST_PATH) {
    return jsonResponse(201, { in_watchlist: true });
  }
  if (method === "DELETE" && url.pathname === `/api/v1/watchlist/${CONTRACT_ID}`) {
    return jsonResponse(200, { in_watchlist: false });
  }
  if (url.pathname === "/health") {
    return jsonResponse(200, { status: "ok" });
  }
  return jsonResponse(500, { error: `unhandled ${method} ${url.pathname}` });
}

/**
 * Runs a message handler with the fake browser and a scripted API in place.
 * @param {object} options
 */
async function withEnvironment(options, run) {
  const browser = installFakeBrowser();
  const fetchStub = installFetch(options.api || defaultApi);
  try {
    if (options.settings) {
      await saveSettings(options.settings);
    }
    return await run({ browser, fetchStub });
  } finally {
    fetchStub.restore();
    browser.restore();
  }
}

test("ping identifies the product", async () => {
  await withEnvironment({}, async () => {
    const response = await handleMessage({ type: MESSAGE.ping }, {});
    assert.deepEqual(response, { ok: true, product: "Sorolens" });
  });
});

test("unsupported message types are rejected", async () => {
  await withEnvironment({}, async () => {
    const response = await handleMessage({ type: "sorolens:nope" }, {});
    assert.equal(response.ok, false);
    assert.match(response.error, /Unsupported message/);
  });
});

test("status rejects a malformed contract id without calling the API", async () => {
  await withEnvironment({}, async ({ fetchStub }) => {
    const response = await handleMessage(
      { type: MESSAGE.status, contractId: "not-an-id" },
      {},
    );
    assert.deepEqual(response, { ok: false, error: "Invalid contract ID" });
    assert.equal(fetchStub.calls.length, 0);
  });
});

test("status reports an indexed contract that is not on the watchlist", async () => {
  await withEnvironment({}, async () => {
    const response = await handleMessage(
      { type: MESSAGE.status, contractId: CONTRACT_ID },
      {},
    );
    assert.equal(response.ok, true);
    assert.equal(response.status.tracked, true);
    assert.equal(response.status.inWatchlist, false);
    assert.equal(response.status.status, "active");
  });
});

test("status caches results until force is requested", async () => {
  await withEnvironment({}, async ({ fetchStub }) => {
    await handleMessage({ type: MESSAGE.status, contractId: CONTRACT_ID }, {});
    const afterFirst = fetchStub.calls.length;
    assert.ok(afterFirst >= 2);

    await handleMessage({ type: MESSAGE.status, contractId: CONTRACT_ID }, {});
    assert.equal(fetchStub.calls.length, afterFirst, "second read must hit the cache");

    await handleMessage(
      { type: MESSAGE.status, contractId: CONTRACT_ID, force: true },
      {},
    );
    assert.ok(fetchStub.calls.length > afterFirst, "force must refetch");
  });
});

test("statusBatch preserves the requested order", async () => {
  const otherId = "C" + "B".repeat(55);
  await withEnvironment(
    {
      api: (request) => {
        if (request.url.pathname === `/api/v1/contracts/${otherId}`) {
          return jsonResponse(404, { error: "not found" });
        }
        return defaultApi(request);
      },
    },
    async () => {
      const response = await handleMessage(
        { type: MESSAGE.statusBatch, contractIds: [CONTRACT_ID, otherId] },
        {},
      );
      assert.deepEqual(
        response.statuses.map((status) => status.contractId),
        [CONTRACT_ID, otherId],
      );
      assert.deepEqual(
        response.statuses.map((status) => status.tracked),
        [true, false],
      );
    },
  );
});

test("track registers and watchlists, then reports the new status", async () => {
  let watched = false;
  await withEnvironment(
    {
      api: (request) => {
        const { method, url } = request;
        if (method === "GET" && url.pathname === CONTRACT_PATH) {
          return jsonResponse(200, { id: CONTRACT_ID, status: "active" });
        }
        if (method === "GET" && url.pathname === STATUS_PATH) {
          return jsonResponse(200, { in_watchlist: watched });
        }
        if (method === "POST" && url.pathname === WATCHLIST_PATH) {
          watched = true;
          return jsonResponse(201, { in_watchlist: true });
        }
        return jsonResponse(500, { error: "unhandled" });
      },
    },
    async () => {
      const response = await handleMessage(
        { type: MESSAGE.track, contractId: CONTRACT_ID },
        {},
      );
      assert.equal(response.ok, true);
      assert.equal(response.outcome.registered, true);
      assert.equal(response.outcome.inWatchlist, true);
      assert.equal(response.status.inWatchlist, true);
    },
  );
});

test("track surfaces a forbidden registration as a warning", async () => {
  await withEnvironment(
    {
      api: (request) => {
        const { method, url } = request;
        if (method === "GET" && url.pathname === CONTRACT_PATH) {
          return jsonResponse(404, { error: "not found" });
        }
        if (method === "POST" && url.pathname === "/api/v1/contracts") {
          return jsonResponse(403, { error: "missing scope" });
        }
        if (method === "POST" && url.pathname === WATCHLIST_PATH) {
          return jsonResponse(201, { in_watchlist: true });
        }
        if (method === "GET" && url.pathname === STATUS_PATH) {
          return jsonResponse(200, { in_watchlist: true });
        }
        return jsonResponse(500, { error: "unhandled" });
      },
    },
    async () => {
      const response = await handleMessage(
        { type: MESSAGE.track, contractId: CONTRACT_ID },
        {},
      );
      assert.equal(response.ok, true);
      assert.equal(response.outcome.registered, false);
      assert.equal(response.outcome.inWatchlist, true);
      assert.equal(response.outcome.warnings.length, 1);
    },
  );
});

test("untrack removes the contract from the watchlist", async () => {
  await withEnvironment({}, async () => {
    const response = await handleMessage(
      { type: MESSAGE.untrack, contractId: CONTRACT_ID },
      {},
    );
    assert.equal(response.ok, true);
    assert.equal(response.status.inWatchlist, false);
  });
});

test("scanResult stores the tab scan and getScan returns it", async () => {
  await withEnvironment({}, async () => {
    const scanned = [CONTRACT_ID];
    const stored = await handleMessage(
      { type: MESSAGE.scanResult, contractIds: scanned, url: "https://stellar.expert/x" },
      { tab: { id: 7, url: "https://stellar.expert/x" } },
    );
    assert.deepEqual(stored, { ok: true });

    const read = await handleMessage({ type: MESSAGE.getScan, tabId: 7 }, {});
    assert.equal(read.ok, true);
    assert.deepEqual(read.scan.contractIds, scanned);

    const missing = await handleMessage({ type: MESSAGE.getScan, tabId: 99 }, {});
    assert.deepEqual(missing.scan, { contractIds: [], url: "" });
  });
});

test("saveSettings and getSettings round-trip without leaking the key to sync", async () => {
  await withEnvironment({}, async ({ browser }) => {
    await handleMessage(
      {
        type: MESSAGE.saveSettings,
        patch: { apiKey: "sl_secret", network: "testnet" },
      },
      {},
    );
    const response = await handleMessage({ type: MESSAGE.getSettings }, {});
    assert.equal(response.settings.apiKey, "sl_secret");
    assert.equal(response.settings.network, "testnet");
    for (const value of browser.sync.values()) {
      assert.notEqual(value, "sl_secret");
    }
  });
});

test("testConnection reports latency when the API answers", async () => {
  await withEnvironment({}, async () => {
    const response = await handleMessage(
      { type: MESSAGE.testConnection },
      {},
    );
    assert.equal(response.ok, true);
    assert.equal(response.apiBaseUrl, API_BASE);
    assert.equal(response.status, "ok");
    assert.equal(typeof response.latencyMs, "number");
  });
});

test("testConnection surfaces an unreachable API", async () => {
  await withEnvironment(
    { api: () => jsonResponse(503, { error: "down" }) },
    async () => {
      const response = await handleMessage({ type: MESSAGE.testConnection }, {});
      assert.equal(response.ok, false);
      assert.match(response.error, /Sorolens API error/);
    },
  );
});
