import { describe, expect, it } from "vitest";
import { escapeHtml, renderMarkdown, safeUrl } from "./markdown";

describe("escapeHtml", () => {
  it("escapes every character that can start markup", () => {
    expect(escapeHtml(`<a href='x' title="y">&</a>`)).toBe(
      "&lt;a href=&#39;x&#39; title=&quot;y&quot;&gt;&amp;&lt;/a&gt;"
    );
  });
});

describe("safeUrl", () => {
  it("allows http, https, mailto and relative links", () => {
    expect(safeUrl("https://example.com/a?b=1")).toBe(
      "https://example.com/a?b=1"
    );
    expect(safeUrl("http://example.com")).toBe("http://example.com");
    expect(safeUrl("mailto:ops@example.com")).toBe("mailto:ops@example.com");
    expect(safeUrl("/contracts")).toBe("/contracts");
    expect(safeUrl("#notes")).toBe("#notes");
  });

  it("refuses javascript, data and other schemes", () => {
    expect(safeUrl("javascript:alert(1)")).toBeNull();
    expect(safeUrl("JavaScript:alert(1)")).toBeNull();
    expect(safeUrl("data:text/html,<script>alert(1)</script>")).toBeNull();
    expect(safeUrl("vbscript:msgbox(1)")).toBeNull();
  });

  it("normalises the control characters browsers strip from URLs", () => {
    // Without normalising, this reads as a relative URL here but resolves as
    // javascript: in the browser.
    expect(safeUrl("java\nscript:alert(1)")).toBeNull();
    expect(safeUrl("java\tscript:alert(1)")).toBeNull();
    expect(safeUrl("java\rscript:alert(1)")).toBeNull();
    expect(safeUrl("\u0000javascript:alert(1)")).toBeNull();
    expect(safeUrl("   ")).toBeNull();
  });
});

describe("renderMarkdown", () => {
  it("renders headings, lists, quotes and rules", () => {
    expect(renderMarkdown("# Title")).toBe("<h1>Title</h1>");
    expect(renderMarkdown("### Deep")).toBe("<h3>Deep</h3>");
    expect(renderMarkdown("- one\n- two")).toBe(
      "<ul>\n<li>one</li>\n<li>two</li>\n</ul>"
    );
    expect(renderMarkdown("1. one\n2. two")).toBe(
      "<ol>\n<li>one</li>\n<li>two</li>\n</ol>"
    );
    expect(renderMarkdown("> quoted")).toBe("<blockquote>quoted</blockquote>");
    expect(renderMarkdown("---")).toBe("<hr />");
  });

  it("renders paragraphs and line breaks", () => {
    expect(renderMarkdown("plain text")).toBe("<p>plain text</p>");
    expect(renderMarkdown("one\ntwo")).toBe("<p>one<br />two</p>");
    expect(renderMarkdown("first\n\nsecond")).toBe(
      "<p>first</p>\n<p>second</p>"
    );
  });

  it("renders emphasis and inline code", () => {
    expect(renderMarkdown("**bold** then *italic*")).toBe(
      "<p><strong>bold</strong> then <em>italic</em></p>"
    );
    expect(renderMarkdown("use `contract_id` here")).toBe(
      "<p>use <code>contract_id</code> here</p>"
    );
  });

  it("renders fenced code blocks verbatim and escaped", () => {
    expect(renderMarkdown("```\nlet x = 1 < 2 && true;\n```")).toBe(
      "<pre><code>let x = 1 &lt; 2 &amp;&amp; true;</code></pre>"
    );
  });

  it("escapes raw HTML instead of interpreting it", () => {
    const html = renderMarkdown('<img src=x onerror="alert(1)">');
    expect(html).toBe("<p>&lt;img src=x onerror=&quot;alert(1)&quot;&gt;</p>");
    expect(html).not.toContain("<img");
  });

  it("escapes script tags and inline handlers in every block type", () => {
    const samples = [
      "<script>alert(1)</script>",
      "# <script>alert(1)</script>",
      "- <script>alert(1)</script>",
      "> <script>alert(1)</script>",
      "**<script>alert(1)</script>**",
    ];
    for (const sample of samples) {
      const html = renderMarkdown(sample);
      expect(html).not.toContain("<script");
      expect(html).toContain("&lt;script&gt;");
    }
  });

  it("escapes HTML inside code spans and code blocks", () => {
    expect(renderMarkdown("`<b>x</b>`")).toContain("&lt;b&gt;x&lt;/b&gt;");
    expect(renderMarkdown("```\n<b>x</b>\n```")).toContain(
      "&lt;b&gt;x&lt;/b&gt;"
    );
  });

  it("renders allowed links with a hardened rel", () => {
    const html = renderMarkdown("[docs](https://example.com/guide)");
    expect(html).toContain('href="https://example.com/guide"');
    expect(html).toContain('rel="noopener noreferrer nofollow"');
    expect(html).toContain(">docs</a>");
  });

  it("drops links whose scheme is not allowed, keeping the text", () => {
    const html = renderMarkdown("[click me](javascript:alert(1))");
    expect(html).not.toContain("<a");
    expect(html).not.toContain("javascript");
    expect(html).toContain("click me");
  });

  it("cannot be broken out of through the link target", () => {
    const html = renderMarkdown(
      '[x](https://example.com" onmouseover="alert(1))'
    );
    expect(html).not.toContain('onmouseover="alert(1)"');
    expect(html).toContain("&quot;");
  });

  it("ignores NUL characters, so a note cannot forge a placeholder", () => {
    // Placeholders use NUL framing, and NUL is stripped from the input first,
    // so text in a note can never be expanded as if the renderer had made it.
    expect(renderMarkdown("\u00000\u0000")).toBe("<p>0</p>");
    expect(renderMarkdown("`x`\u00000\u0000")).toBe("<p><code>x</code>0</p>");
  });

  it("returns a safe empty string for empty input", () => {
    expect(renderMarkdown("")).toBe("");
    expect(renderMarkdown("\n\n")).toBe("");
  });
});
