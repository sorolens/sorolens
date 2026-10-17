import { describe, expect, it } from "vitest";
import sitemap from "./sitemap";

describe("sitemap", () => {
  it("includes the documentation route", () => {
    expect(sitemap().map((entry) => new URL(entry.url).pathname)).toContain(
      "/docs"
    );
  });
});
