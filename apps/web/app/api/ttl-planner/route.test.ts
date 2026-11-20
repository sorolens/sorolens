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
});
