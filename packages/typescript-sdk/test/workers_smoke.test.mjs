import assert from "node:assert/strict";
import { after, test } from "node:test";
import { Miniflare } from "miniflare";

const mf = new Miniflare({
  modules: true,
  scriptPath: "test/workers_smoke.mjs",
  compatibilityDate: "2025-01-01",
  // The built bundle uses .js for ESM (as package.json exports declare).
  modulesRules: [{ type: "ESModule", include: ["**/*.js"], fallthrough: true }],
});

after(() => mf.dispose());

test("webhook verification runs inside workerd", async () => {
  const pinned = "t=1700000000,v1=97dc0f80c8e0c9be056f34e48e360411160395a1b3e3a7978092d869315c4615";
  const response = await mf.dispatchFetch("http://localhost/webhook", {
    headers: { "X-Sorolens-Signature": pinned, "X-Sorolens-Timestamp": "1700000000" },
  });
  assert.deepEqual(await response.json(), { valid: true });
});

test("client retries and paginates inside workerd", async () => {
  const response = await mf.dispatchFetch("http://localhost/contracts");
  assert.deepEqual(await response.json(), { ids: ["C1", "C2"], calls: 3 });
});
