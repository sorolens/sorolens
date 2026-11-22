/** Tests for the in-app documentation loader (issue #598). */
import { describe, expect, it } from "vitest";
import { listDocs, renderDoc, renderMarkdownPage } from "./docs";
import sitemap from "@/app/sitemap";

describe("in-app documentation", () => {
  it("lists the getting-started and concepts sections", () => {
    const slugs = listDocs().map((doc) => doc.slug);
    expect(slugs).toContain("getting-started");
    expect(slugs).toContain("concepts");
  });

  it("renders every listed doc from repo markdown", () => {
    for (const doc of listDocs()) {
      const html = renderDoc(doc.slug);
      expect(html, doc.slug).toBeTruthy();
      expect(html).toContain("<h1>");
      expect(html).not.toContain("<script");
    }
  });

  it("renders the about page from repo markdown", () => {
    const html = renderMarkdownPage("about");
    expect(html).toContain("<h1>About Sorolens</h1>");
  });

  it("returns undefined for an unknown doc", () => {
    expect(renderDoc("not-a-real-page")).toBeUndefined();
  });

  it("includes docs and about in the sitemap", () => {
    const urls = sitemap().map((entry) => entry.url);
    for (const path of [
      "/docs",
      "/docs/getting-started",
      "/docs/concepts",
      "/about",
    ]) {
      expect(urls.some((url) => url.endsWith(path))).toBe(true);
    }
  });
});
