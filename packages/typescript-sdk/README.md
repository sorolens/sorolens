# @sorolens/sdk

Edge-runtime compatible TypeScript client for the [Sorolens](../../) API.

The same build runs unchanged on **Cloudflare Workers**, **Deno**, **Bun**, the
**browser** and Node 18+: it uses only `fetch`, `AbortController`, `URL`,
`TextDecoder` and Web Crypto. There are no Node built-ins, no polyfills and no
runtime dependencies.

- Dual **ESM + CJS** builds with a correct `exports` map and `sideEffects: false`
  for tree-shaking.
- Retries for safe requests on 429/5xx, honouring `Retry-After`.
- `AsyncGenerator` pagination and a reconnecting SSE stream with
  `Last-Event-ID` resume and de-duplication.
- `verifyWebhook` on Web Crypto: constant-time comparison and a replay window.

## Install

```bash
npm install @sorolens/sdk
```

## Quickstart

```ts
import { SorolensClient } from "@sorolens/sdk";

const client = new SorolensClient({
  apiKey: "sl_live_...",
  retry: { maxRetries: 3 },
});

const stats = await client.globalStats();
console.log(stats.tracked_contracts);

for await (const contract of client.contracts({ limit: 100 })) {
  console.log(contract.id);
}

for await (const event of client.events(contractId, { limit: 100 })) {
  console.log(event.id);
}
```

All methods accept an injected `fetch` for tests or custom runtimes:

```ts
const client = new SorolensClient({ fetch: myFetch });
```

### Streaming

```ts
const controller = new AbortController();

for await (const event of client.streamEvents({
  contractId,
  signal: controller.signal,
  heartbeatTimeoutMs: 45_000,
})) {
  console.log(event.id, event.type, event.data);
}
```

The stream reconnects with exponential backoff and jitter, sends the standard
`Last-Event-ID` header and remembers the last 256 ids so replayed events are
not yielded twice. Delivery is **at-least-once** on the wire, **at-most-once**
through this iterator inside the id window; 401/403 end the iterator with a
`SorolensError` instead of retrying forever.

### Webhooks

```ts
import { verifyWebhook } from "@sorolens/sdk";

const ok = await verifyWebhook({
  secret: env.SOROLENS_WEBHOOK_SECRET,
  signature: request.headers.get("X-Sorolens-Signature"),
  timestamp: request.headers.get("X-Sorolens-Timestamp"),
  body: rawBody, // the exact bytes, before JSON.parse
});

if (!ok) return new Response("invalid signature", { status: 401 });
```

The published vector from [`docs/webhooks.md`](../../docs/webhooks.md) is part of
the test suite.

## Edge compatibility

| Runtime            | Verified by                                                    |
| ------------------ | -------------------------------------------------------------- |
| Node 18+           | `node --test test/*.test.mjs`                                   |
| Deno               | `deno test test/deno_smoke.ts`                                  |
| Cloudflare Workers | Miniflare/workerd smoke test (`test/workers_smoke.test.mjs`)    |
| Browser            | No Node APIs in the bundle (enforced by `test/edge-safety`)      |

```bash
npm test                      # build + Node, edge-safety and workerd tests
npx deno test test/deno_smoke.ts
```

## Bundle size and tree-shaking

`npm run size` builds a minified bundle and reports:

```
full bundle:  6.18 kB min, 2.69 kB gzip
verifyWebhook only: 1.07 kB min
tree-shaking: ok (client code dropped)
```

Reported on the pull request that introduced the package. The SDK is published
as ESM with `sideEffects: false`, so importing only `verifyWebhook` drops the
client entirely.

## Example

[`examples/worker.ts`](examples/worker.ts) is a complete Cloudflare Worker that
verifies watchdog webhooks and lists contracts.
