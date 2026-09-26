/**
 * Guards the iOS home-screen icon added in #220: the asset must ship at
 * exactly 180x180 as an opaque PNG, and the root layout must reference it so
 * Next.js emits <link rel="apple-touch-icon" ... />.
 */
import { existsSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it, vi } from "vitest";
import type { Icons } from "next/dist/lib/metadata/types/metadata-types";
import { metadata } from "./layout";

// The layout pulls in the Tailwind entrypoint; it is irrelevant here.
vi.mock("./globals.css", () => ({}));

const here = path.dirname(fileURLToPath(import.meta.url));
const publicPath = path.join(here, "..", "public");
const iconPath = path.join(publicPath, "apple-touch-icon.png");

const readManifest = () =>
  JSON.parse(readFileSync(path.join(publicPath, "manifest.json"), "utf8"));

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
    expect((metadata.icons as Icons)?.apple).toEqual([
      { url: "/apple-touch-icon.png", sizes: "180x180", type: "image/png" },
    ]);
  });

  it("links an installable web app manifest from the root layout", () => {
    expect(metadata.manifest).toBe("/manifest.json");

    const manifest = readManifest();
    expect(manifest).toMatchObject({
      name: "Sorolens — Indexed Observability for Soroban",
      short_name: "Sorolens",
      start_url: "/",
      display: "standalone",
      theme_color: "#06b6d4",
      background_color: "#11111b",
      prefer_related_applications: false,
    });
  });

  it("provides 192px and 512px PNG icons declared by the manifest", () => {
    const manifest = readManifest();
    const requiredSizes = ["192x192", "512x512"];

    for (const size of requiredSizes) {
      const icon = manifest.icons.find(
        (entry: { sizes: string }) => entry.sizes === size,
      );
      if (!icon) throw new Error(`manifest icon for ${size} is missing`);
      expect(icon.type).toBe("image/png");

      const png = readFileSync(
        path.join(publicPath, icon.src.replace(/^\//, "")),
      );
      expect(png.subarray(0, 8).toString("hex")).toBe("89504e470d0a1a0a");
      expect(png.readUInt32BE(16)).toBe(Number(size.split("x")[0]));
      expect(png.readUInt32BE(20)).toBe(Number(size.split("x")[1]));
    }
  });
});
