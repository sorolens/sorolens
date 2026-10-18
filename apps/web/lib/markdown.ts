/**
 * A deliberately small markdown renderer for contract notes (issue #164).
 *
 * Notes are authored by users and rendered into the dashboard, so this module
 * is built the safe way round: every character of the input is HTML-escaped,
 * and the only tags that ever reach the output are the ones this file emits
 * itself. Raw HTML in a note is therefore shown as text instead of being
 * interpreted, which is what makes the rendering XSS-safe without pulling in a
 * sanitiser dependency.
 *
 * Supported: headings, unordered and ordered lists, blockquotes, fenced and
 * indented code blocks, horizontal rules, inline code, bold, italic, and links
 * whose scheme is http, https or mailto (everything else, `javascript:` and
 * `data:` included, is rendered as plain text).
 *
 * Not supported on purpose: raw HTML passthrough, underscore emphasis (it
 * breaks identifiers such as `contract_id`), images, and tables.
 */

const ESCAPE_MAP: Record<string, string> = {
  "&": "&amp;",
  "<": "&lt;",
  ">": "&gt;",
  '"': "&quot;",
  "'": "&#39;",
};

/** escapeHtml replaces every character that could start markup. */
export function escapeHtml(value: string): string {
  return value.replace(/[&<>"']/g, (c) => ESCAPE_MAP[c]);
}

/**
 * safeUrl returns a normalised URL when its scheme is allowed, or null.
 *
 * Browsers strip tabs and newlines out of URLs before resolving them, so
 * `java\nscript:alert(1)` would otherwise look like a relative URL here and
 * then execute in the page. Normalising first closes that gap, and anything
 * whose scheme is not on the allow-list is refused outright.
 */
export function safeUrl(raw: string): string | null {
  const url = raw.replace(/[\t\n\r]/g, "").trim();
  if (url === "") return null;
  // Control characters have no business in a link and can confuse parsers.
  if (/[\u0000-\u001f\u007f]/.test(url)) return null;
  const scheme = /^([a-zA-Z][a-zA-Z0-9+.-]*):/.exec(url);
  if (scheme && !/^(https?|mailto)$/i.test(scheme[1])) return null;
  return url;
}

// BLOCK_START matches a line that begins a block-level construct, which is how
// a paragraph knows where to stop.
const BLOCK_START =
  /^\s*(```|#{1,6}\s|[-*+]\s|\d+[.)]\s|>|-{3,}\s*$|\*{3,}\s*$|_{3,}\s*$)/;

/** renderInline applies the inline rules to one span of text. */
function renderInline(source: string): string {
  const tokens: string[] = [];
  // Placeholders are kept out of the escaping pass by using a character that
  // cannot survive it, and any literal NUL in the note is dropped first so a
  // note cannot forge a placeholder of its own.
  const keep = (html: string) => {
    tokens.push(html);
    return `\u0000${tokens.length - 1}\u0000`;
  };

  let text = source.replace(/\u0000/g, "");

  // Code spans are tokenised first so emphasis and link rules never look
  // inside them.
  text = text.replace(/`([^`\n]+)`/g, (_match, code: string) =>
    keep(`<code>${escapeHtml(code)}</code>`)
  );

  text = text.replace(
    /\[([^\]\n]+)\]\(([^)\n]+)\)/g,
    (_match, label: string, rawUrl: string) => {
      const url = safeUrl(rawUrl);
      const labelHtml = escapeHtml(label);
      if (!url) return labelHtml;
      return keep(
        `<a href="${escapeHtml(url)}" target="_blank" rel="noopener noreferrer nofollow">${labelHtml}</a>`
      );
    }
  );

  text = escapeHtml(text);
  text = text.replace(/\*\*([^*\n]+)\*\*/g, "<strong>$1</strong>");
  text = text.replace(/(^|[^*])\*([^*\n]+)\*/g, "$1<em>$2</em>");
  text = text.replace(/\n/g, "<br />");

  return text.replace(/\u0000(\d+)\u0000/g, (_match, index: string) => {
    return tokens[Number(index)] ?? "";
  });
}

/**
 * renderMarkdown turns markdown source into an HTML string.
 *
 * The result is safe to hand to `dangerouslySetInnerHTML`: it is assembled
 * exclusively from escaped text and tags this module generates.
 */
export function renderMarkdown(source: string): string {
  const lines = String(source ?? "")
    .replace(/\u0000/g, "")
    .replace(/\r\n?/g, "\n")
    .split("\n");

  const out: string[] = [];
  let list: "ul" | "ol" | null = null;
  const closeList = () => {
    if (list) {
      out.push(`</${list}>`);
      list = null;
    }
  };

  let i = 0;
  while (i < lines.length) {
    const line = lines[i];

    if (/^\s*```/.test(line)) {
      closeList();
      const code: string[] = [];
      i += 1;
      while (i < lines.length && !/^\s*```/.test(lines[i])) {
        code.push(lines[i]);
        i += 1;
      }
      i += 1; // Consume the closing fence, or accept an unclosed block.
      out.push(`<pre><code>${escapeHtml(code.join("\n"))}</code></pre>`);
      continue;
    }

    if (/^\s*$/.test(line)) {
      closeList();
      i += 1;
      continue;
    }

    if (/^\s*(-{3,}|\*{3,}|_{3,})\s*$/.test(line)) {
      closeList();
      out.push("<hr />");
      i += 1;
      continue;
    }

    const heading = /^(#{1,6})\s+(.*)$/.exec(line);
    if (heading) {
      closeList();
      const level = heading[1].length;
      out.push(`<h${level}>${renderInline(heading[2].trim())}</h${level}>`);
      i += 1;
      continue;
    }

    const bullet = /^\s*[-*+]\s+(.*)$/.exec(line);
    const numbered = /^\s*\d+[.)]\s+(.*)$/.exec(line);
    if (bullet || numbered) {
      const kind: "ul" | "ol" = bullet ? "ul" : "ol";
      if (list !== kind) {
        closeList();
        out.push(`<${kind}>`);
        list = kind;
      }
      out.push(`<li>${renderInline((bullet ?? numbered)![1])}</li>`);
      i += 1;
      continue;
    }

    const quote = /^\s*>\s?(.*)$/.exec(line);
    if (quote) {
      closeList();
      out.push(`<blockquote>${renderInline(quote[1])}</blockquote>`);
      i += 1;
      continue;
    }

    // Paragraph: run on until a blank line or the next block-level construct.
    const paragraph = [line];
    i += 1;
    while (
      i < lines.length &&
      !/^\s*$/.test(lines[i]) &&
      !BLOCK_START.test(lines[i])
    ) {
      paragraph.push(lines[i]);
      i += 1;
    }
    closeList();
    out.push(`<p>${renderInline(paragraph.join("\n"))}</p>`);
  }

  closeList();
  return out.join("\n");
}
