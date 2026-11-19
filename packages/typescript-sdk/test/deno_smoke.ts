// Deno smoke test. Run after building:
//
//   npm run build
//   deno test --allow-net test/deno_smoke.ts
//
// It proves the published ESM bundle imports and its Web Crypto / fetch code
// paths work outside Node.
import { SorolensClient, verifyWebhook } from "../dist/esm/index.js";

const SECRET = "whsec_example_0123456789abcdef0123456789abcdef";
const BODY = '{"event":"watchdog.alert","severity":"Critical"}';
const SIGNATURE =
  "t=1700000000,v1=97dc0f80c8e0c9be056f34e48e360411160395a1b3e3a7978092d869315c4615";

Deno.test("webhook verification matches the shared vector", async () => {
  const ok = await verifyWebhook({
    secret: SECRET,
    signature: SIGNATURE,
    timestamp: "1700000000",
    body: BODY,
    nowSeconds: 1700000000,
  });
  if (!ok) throw new Error("expected the shared vector to verify");
});

Deno.test("client retries and paginates against a mock fetch", async () => {
  let calls = 0;
  const fetchImpl = async (input: string | URL | Request) => {
    const url = new URL(String(input));
    calls += 1;
    if (calls === 1) {
      return new Response("{}", { status: 429, headers: { "Retry-After": "0" } });
    }
    if (url.searchParams.get("cursor") === "p2") {
      return Response.json({ contracts: [{ id: "C2" }], next_cursor: "" });
    }
    return Response.json({ contracts: [{ id: "C1" }], next_cursor: "p2" });
  };

  const client = new SorolensClient({
    baseUrl: "https://api.test",
    fetch: fetchImpl as typeof fetch,
    retry: { baseDelayMs: 1 },
  });
  const ids: string[] = [];
  for await (const contract of client.contracts()) ids.push(contract.id as string);
  if (ids.join(",") !== "C1,C2") throw new Error(`unexpected ids: ${ids}`);
  if (calls !== 3) throw new Error(`expected 3 requests, got ${calls}`);
});
