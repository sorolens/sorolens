// Minimal, dependency-free markdown parser used to render contract notes.
//
// Safety model: this module never emits raw HTML. It turns markdown into a
// small, closed set of typed tokens that the renderer maps to React elements,
// so user input is always rendered as text (React escapes it). Link targets
// are additionally restricted to http(s), mailto, in-page anchors, and
// same-origin relative paths, which blocks `javascript:`/`data:` payloads.

export type InlineToken =
  | { type: "text"; value: string }
  | { type: "code"; value: string }
  | { type: "strong"; value: string }
  | { type: "em"; value: string }
  | { type: "link"; value: string; href: string };

export type MarkdownBlock =
  | { type: "heading"; level: 1 | 2 | 3; inline: InlineToken[] }
  | { type: "paragraph"; inline: InlineToken[] }
  | { type: "code"; value: string }
  | { type: "list"; ordered: boolean; items: InlineToken[][] }
  | { type: "quote"; inline: InlineToken[] };

const SAFE_PROTOCOLS = new Set(["http:", "https:", "mailto:"]);

/**
 * sanitizeUrl returns a safe href for a markdown link target, or null when
 * the target uses a scheme we do not allow (javascript:, data:, vbscript:,
 * protocol-relative //host, …). Rejected targets are rendered as plain text.
 */
export function sanitizeUrl(raw: string): string | null {
  const url = raw.trim();
  if (url === "") return null;

  // In-page anchors and same-origin relative paths.
  if (url.startsWith("#")) return url;
  if (url.startsWith("/") && !url.startsWith("//")) return url;

  try {
    const parsed = new URL(url);
    if (SAFE_PROTOCOLS.has(parsed.protocol)) return url;
  } catch {
    // Not an absolute URL and not relative/anchor: treat as unsafe/unknown.
  }
  return null;
}

// Order matters: `**bold**` is tried before `*italic*` so the double marker
// wins at a shared start position.
const INLINE_RE =
  /(`[^`]+`)|(\*\*[^*]+\*\*)|(__[^_]+__)|(\*[^*]+\*)|(_[^_]+_)|(\[[^\]]+\]\([^)\s]+\))/;

/** parseInline splits a single line into formatted spans. */
export function parseInline(text: string): InlineToken[] {
  const tokens: InlineToken[] = [];
  let rest = text;

  while (rest.length > 0) {
    const m = INLINE_RE.exec(rest);
    if (!m) {
      tokens.push({ type: "text", value: rest });
      break;
    }

    if (m.index > 0) {
      tokens.push({ type: "text", value: rest.slice(0, m.index) });
    }

    const match = m[0];
    if (match.startsWith("`")) {
      tokens.push({ type: "code", value: match.slice(1, -1) });
    } else if (match.startsWith("**")) {
      tokens.push({ type: "strong", value: match.slice(2, -2) });
    } else if (match.startsWith("__")) {
      tokens.push({ type: "strong", value: match.slice(2, -2) });
    } else if (match.startsWith("*")) {
      tokens.push({ type: "em", value: match.slice(1, -1) });
    } else if (match.startsWith("_")) {
      tokens.push({ type: "em", value: match.slice(1, -1) });
    } else {
      const close = match.lastIndexOf("](");
      const label = match.slice(1, close);
      const href = sanitizeUrl(match.slice(close + 2, -1));
      if (href) {
        tokens.push({ type: "link", value: label, href });
      } else {
        tokens.push({ type: "text", value: label });
      }
    }

    rest = rest.slice(m.index + match.length);
  }

  return tokens;
}

const HEADING_RE = /^(#{1,3})\s+(.*)$/;
const LIST_RE = /^\s*(?:([-*+])|(\d+)\.)\s+(.*)$/;

/** parseMarkdown turns a markdown document into renderable blocks. */
export function parseMarkdown(md: string): MarkdownBlock[] {
  const lines = md.replace(/\r\n?/g, "\n").split("\n");
  const blocks: MarkdownBlock[] = [];
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];

    if (line.trimStart().startsWith("```")) {
      const code: string[] = [];
      i++;
      while (i < lines.length && !lines[i].trimStart().startsWith("```")) {
        code.push(lines[i]);
        i++;
      }
      i++; // consume the closing fence (or run off the end)
      blocks.push({ type: "code", value: code.join("\n") });
      continue;
    }

    if (line.trim() === "") {
      i++;
      continue;
    }

    const heading = HEADING_RE.exec(line);
    if (heading) {
      blocks.push({
        type: "heading",
        level: heading[1].length as 1 | 2 | 3,
        inline: parseInline(heading[2].trim()),
      });
      i++;
      continue;
    }

    if (line.trimStart().startsWith(">")) {
      const quote: string[] = [];
      while (i < lines.length && lines[i].trimStart().startsWith(">")) {
        quote.push(lines[i].trimStart().replace(/^>\s?/, ""));
        i++;
      }
      blocks.push({ type: "quote", inline: parseInline(quote.join(" ").trim()) });
      continue;
    }

    const list = LIST_RE.exec(line);
    if (list) {
      const ordered = list[2] !== undefined;
      const items: InlineToken[][] = [];
      while (i < lines.length) {
        const item = LIST_RE.exec(lines[i]);
        if (!item) break;
        items.push(parseInline(item[3].trim()));
        i++;
      }
      blocks.push({ type: "list", ordered, items });
      continue;
    }

    const para: string[] = [];
    while (i < lines.length) {
      const l = lines[i];
      if (
        l.trim() === "" ||
        HEADING_RE.test(l) ||
        LIST_RE.test(l) ||
        l.trimStart().startsWith(">") ||
        l.trimStart().startsWith("```")
      ) {
        break;
      }
      para.push(l.trim());
      i++;
    }
    if (para.length > 0) {
      blocks.push({ type: "paragraph", inline: parseInline(para.join(" ")) });
      continue;
    }

    // Defensive: a line we could not classify still advances the cursor.
    i++;
  }

  return blocks;
}
