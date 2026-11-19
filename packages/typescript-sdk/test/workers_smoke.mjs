// Cloudflare Workers (workerd) smoke module, exercised by workers_smoke.test.mjs.
import { SorolensClient, verifyWebhook } from "../dist/esm/index.js";

const SECRET = "whsec_example_0123456789abcdef0123456789abcdef";
const BODY = '{"event":"watchdog.alert","severity":"Critical"}';
const SIGNATURE =
  "t=1700000000,v1=97dc0f80c8e0c9be056f34e48e360411160395a1b3e3a7978092d869315c4615";

export default {
  async fetch(request) {
    const url = new URL(request.url);

    if (url.pathname === "/webhook") {
      const valid = await verifyWebhook({
        secret: SECRET,
        signature: request.headers.get("X-Sorolens-Signature"),
        timestamp: request.headers.get("X-Sorolens-Timestamp"),
        body: BODY,
        nowSeconds: 1700000000,
      });
      return Response.json({ valid });
    }

    if (url.pathname === "/contracts") {
      let calls = 0;
      const fetchImpl = async (input) => {
        calls += 1;
        const target = new URL(String(input));
        if (calls === 1) return new Response("{}", { status: 429, headers: { "Retry-After": "0" } });
        if (target.searchParams.get("cursor") === "p2") {
          return Response.json({ contracts: [{ id: "C2" }], next_cursor: "" });
        }
        return Response.json({ contracts: [{ id: "C1" }], next_cursor: "p2" });
      };
      const client = new SorolensClient({
        baseUrl: "https://api.test",
        fetch: fetchImpl,
        retry: { baseDelayMs: 1 },
      });
      const ids = [];
      for await (const contract of client.contracts()) ids.push(contract.id);
      return Response.json({ ids, calls });
    }

    return new Response("not found", { status: 404 });
  },
};
