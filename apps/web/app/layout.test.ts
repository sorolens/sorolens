/**
 * Guards the iOS home-screen icon added in #220: the asset must ship at
 * exactly 180x180 as an opaque PNG, and the root layout must reference it so
 * Next.js emits <link rel="apple-touch-icon" ... />.
 */
import { existsSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it, vi } from "vitest";
import { metadata } from "./layout";

// The layout pulls in the Tailwind entrypoint; it is irrelevant here.
vi.mock("./globals.css", () => ({}));

const here = path.dirname(fileURLToPath(import.meta.url));
const iconPath = path.join(here, "..", "public", "apple-touch-icon.png");

describe("apple-touch-icon (#220)", () => {
  it("ships a 180x180 PNG in apps/web/public", () => {
    expect(existsSync(iconPath)).toBe(true);

    const png = readFileSync(iconPath);

    // PNG magic bytes.
    expect(png.subarray(0, 8).toString("hex")).toBe("89504e470d0a1a0a");
    // IHDR width/height are big-endian uint32 at offsets 16 and 20.
    expect(png.readUInt32BE(16)).toBe(180);
    expect(png.readUInt32BE(20)).toBe(180);
  });

  it("is referenced from the root layout metadata", () => {
    expect(metadata.icons?.apple).toEqual([
      { url: "/apple-touch-icon.png", sizes: "180x180", type: "image/png" },
    ]);
  });
});
