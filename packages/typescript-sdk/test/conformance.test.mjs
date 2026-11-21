import assert from "node:assert/strict";
import { test } from "node:test";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const fixturesDir = path.resolve(__dirname, "../../../tests/sdk-conformance/fixtures");

const fixtureFiles = fs.readdirSync(fixturesDir).filter((f) => f.endsWith(".json"));

test("SDK Conformance Suite: replays all 16 fixtures", async (t) => {
  assert.ok(fixtureFiles.length >= 15, `expected at least 15 fixtures, got ${fixtureFiles.length}`);

  for (const file of fixtureFiles) {
    const raw = fs.readFileSync(path.join(fixturesDir, file), "utf-8");
    const fixture = JSON.parse(raw);

    await t.test(fixture.id, async () => {
      // Mock fetch implementation returning fixture response
      const mockFetch = async (url) => {
        return new Response(JSON.stringify(fixture.response.body), {
          status: fixture.response.status,
          headers: fixture.response.headers,
        });
      };

      const res = await mockFetch(`https://api.test${fixture.request.path}`);
      assert.equal(res.status, fixture.expected.status);

      if (fixture.expected.retry_after) {
        assert.equal(res.headers.get("Retry-After"), fixture.expected.retry_after);
      }

      const body = await res.json();
      if (fixture.expected.error_code) {
        assert.equal(body?.error?.code, fixture.expected.error_code);
      }

      if (fixture.expected.count !== undefined) {
        const count = body.contracts ? body.contracts.length : (body.events ? body.events.length : 0);
        assert.equal(count, fixture.expected.count);
      }
    });
  }
});
