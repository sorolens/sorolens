import assert from "node:assert/strict";
import { test } from "node:test";
import { SorolensClient, SorolensError } from "../dist/esm/index.js";

const STATS = { tracked_contracts: 42 };

function json(body, status = 200, headers = {}) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json", ...headers },
  });
}

test("retries 429 and honours Retry-After", async () => {
  let calls = 0;
  const fetchImpl = async () => {
    calls += 1;
    if (calls <= 2) return json({ error: { code: "RATE_LIMITED" } }, 429, { "Retry-After": "0" });
    return json(STATS);
  };

  const client = new SorolensClient({ baseUrl: "https://api.test", fetch: fetchImpl });
  const stats = await client.globalStats();
  assert.equal(stats.tracked_contracts, 42);
  assert.equal(calls, 3);
});

test("permanent errors are not retried", async () => {
  let calls = 0;
  const fetchImpl = async () => {
    calls += 1;
    return json({ error: { code: "UNAUTHORIZED", message: "bad key" } }, 401);
  };

  const client = new SorolensClient({ baseUrl: "https://api.test", fetch: fetchImpl });
  await assert.rejects(
    () => client.globalStats(),
    (error) => error instanceof SorolensError && error.status === 401 && error.permanent,
  );
  assert.equal(calls, 1);
});

test("retries can be disabled", async () => {
  let calls = 0;
  const fetchImpl = async () => {
    calls += 1;
    return json({ error: "nope" }, 503);
  };
  const client = new SorolensClient({ baseUrl: "https://api.test", fetch: fetchImpl, retry: false });
  await assert.rejects(() => client.globalStats());
  assert.equal(calls, 1);
});

test("async iterators follow next_cursor", async () => {
  const urls = [];
  const fetchImpl = async (input) => {
    const url = new URL(String(input));
    urls.push(url);
    if (url.searchParams.get("cursor") === "page-2") {
      return json({ contracts: [{ id: "C3" }], next_cursor: "" });
    }
    return json({ contracts: [{ id: "C1" }, { id: "C2" }], next_cursor: "page-2" });
  };

  const client = new SorolensClient({ baseUrl: "https://api.test", fetch: fetchImpl });
  const ids = [];
  for await (const contract of client.contracts({ limit: 2 })) ids.push(contract.id);
  assert.deepEqual(ids, ["C1", "C2", "C3"]);
  assert.equal(urls[0].searchParams.get("limit"), "2");
  assert.equal(urls[1].searchParams.get("cursor"), "page-2");
});

test("streams SSE and skips replayed ids", async () => {
  const encoder = new TextEncoder();
  const chunks = [
    "id: 1\ndata: {\"n\":1}\n\n",
    "id: 2\ndata: {\"n\":2}\n\n",
    "id: 2\ndata: {\"n\":2}\n\n",
    "id: 3\ndata: {\"n\":3}\n\n",
  ];
  let aborted = false;
  const fetchImpl = async (input, init) => {
    const stream = new ReadableStream({
      start(controller) {
        for (const chunk of chunks) controller.enqueue(encoder.encode(chunk));
        init.signal.addEventListener("abort", () => {
          aborted = true;
          try {
            controller.close();
          } catch {
            // already closed
          }
        });
      },
    });
    return new Response(stream, { status: 200, headers: { "content-type": "text/event-stream" } });
  };

  const controller = new AbortController();
  const client = new SorolensClient({ baseUrl: "https://api.test", fetch: fetchImpl });
  const ids = [];
  for await (const event of client.streamEvents({ signal: controller.signal })) {
    ids.push(event.id);
    if (ids.length === 3) controller.abort();
  }
  assert.deepEqual(ids, ["1", "2", "3"]);
  assert.equal(aborted, true);
});
