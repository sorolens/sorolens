import { describe, expect, it } from "vitest";
import { parseInline, parseMarkdown, sanitizeUrl } from "./markdown";

describe("sanitizeUrl", () => {
  it("allows http, https, mailto, anchors and relative paths", () => {
    expect(sanitizeUrl("https://example.com/docs")).toBe(
      "https://example.com/docs",
    );
    expect(sanitizeUrl("http://example.com")).toBe("http://example.com");
    expect(sanitizeUrl("mailto:team@example.com")).toBe(
      "mailto:team@example.com",
    );
    expect(sanitizeUrl("#section")).toBe("#section");
    expect(sanitizeUrl("/contracts/abc")).toBe("/contracts/abc");
  });

  it("rejects script and data payloads", () => {
    expect(sanitizeUrl("javascript:alert(1)")).toBeNull();
    expect(sanitizeUrl("JavaScript:alert(1)")).toBeNull();
    expect(sanitizeUrl("data:text/html;base64,PHNjcmlwdD4=")).toBeNull();
    expect(sanitizeUrl("//evil.example.com")).toBeNull();
    expect(sanitizeUrl("")).toBeNull();
  });
});

describe("parseInline", () => {
  it("formats bold, emphasis and code", () => {
    const tokens = parseInline("**bold** and *italic* and `code`");
    expect(tokens).toEqual([
      { type: "strong", value: "bold" },
      { type: "text", value: " and " },
      { type: "em", value: "italic" },
      { type: "text", value: " and " },
      { type: "code", value: "code" },
    ]);
  });

  it("keeps unsafe link targets as plain text", () => {
    const tokens = parseInline("[click](javascript:evil)");
    expect(tokens).toEqual([{ type: "text", value: "click" }]);
  });

  it("keeps safe links", () => {
    const tokens = parseInline("[docs](https://example.com)");
    expect(tokens).toEqual([
      { type: "link", value: "docs", href: "https://example.com" },
    ]);
  });
});

describe("parseMarkdown", () => {
  it("parses headings, lists, code fences and paragraphs", () => {
    const blocks = parseMarkdown(
      [
        "## Migration",
        "",
        "- moved from v1",
        "- on 2026-08-10",
        "",
        "```",
        "sorolens track CABC",
        "```",
        "",
        "Regular paragraph.",
      ].join("\n"),
    );

    expect(blocks[0]).toEqual({
      type: "heading",
      level: 2,
      inline: [{ type: "text", value: "Migration" }],
    });
    expect(blocks[1].type).toBe("list");
    expect(blocks[2]).toEqual({
      type: "code",
      value: "sorolens track CABC",
    });
    expect(blocks[3].type).toBe("paragraph");
  });
});
