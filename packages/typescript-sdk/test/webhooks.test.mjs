import assert from "node:assert/strict";
import { test } from "node:test";
import { verifyWebhook } from "../dist/esm/index.js";

const SECRET = "whsec_example_0123456789abcdef0123456789abcdef";
const BODY = '{"event":"watchdog.alert","severity":"Critical"}';
const SIGNATURE =
  "t=1700000000,v1=97dc0f80c8e0c9be056f34e48e360411160395a1b3e3a7978092d869315c4615";

const verify = (overrides = {}) =>
  verifyWebhook({
    secret: SECRET,
    signature: SIGNATURE,
    timestamp: "1700000000",
    body: BODY,
    nowSeconds: 1700000000,
    ...overrides,
  });

test("accepts the published test vector", async () => {
  assert.equal(await verify(), true);
});

test("accepts any matching v1 signature during rotation", async () => {
  const rotated = SIGNATURE.replace("v1=", "v1=0000,v1=");
  assert.equal(await verify({ signature: rotated }), true);
});

test("rejects a tampered body", async () => {
  assert.equal(await verify({ body: BODY.replace("Critical", "Info") }), false);
});

test("rejects a stale timestamp", async () => {
  assert.equal(await verify({ nowSeconds: 1700001000 }), false);
});

test("rejects a mismatched timestamp header", async () => {
  assert.equal(await verify({ timestamp: "1700000001" }), false);
});

test("rejects a malformed signature", async () => {
  assert.equal(await verify({ signature: "t=1700000000,v1=not-hex" }), false);
  assert.equal(await verify({ signature: null }), false);
});

test("accepts the body as bytes", async () => {
  assert.equal(await verify({ body: new TextEncoder().encode(BODY) }), true);
});
