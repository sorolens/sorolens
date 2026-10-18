import { test, expect } from "@playwright/test";

// Issue #230: legacy browsers and some tooling request /favicon.ico directly.
test("/favicon.ico is served as a valid multi-size ICO", async ({
  request,
}) => {
  const res = await request.get("/favicon.ico");

  expect(res.status()).toBe(200);
  expect(res.headers()["content-type"] ?? "").toContain("image");

  const body = await res.body();

  // ICO header: reserved (0), type (1 = icon), image count.
  expect(body.readUInt16LE(0)).toBe(0);
  expect(body.readUInt16LE(2)).toBe(1);

  const count = body.readUInt16LE(4);
  expect(count).toBeGreaterThanOrEqual(3);

  // Each 16-byte directory entry starts with width then height (0 means 256).
  const sizes = new Set<number>();
  for (let i = 0; i < count; i += 1) {
    const entry = 6 + i * 16;
    const width = body.readUInt8(entry) || 256;
    const height = body.readUInt8(entry + 1) || 256;
    expect(width).toBe(height);
    sizes.add(width);
  }

  expect([...sizes].sort((a, b) => a - b)).toEqual([16, 32, 48]);
});

test("HTML head advertises both the ICO and the SVG icon", async ({
  request,
}) => {
  const html = await (await request.get("/")).text();

  expect(html).toContain("/favicon.ico");
  expect(html).toContain("/favicon.svg");
});
