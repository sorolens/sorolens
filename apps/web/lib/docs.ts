import { readFileSync } from "node:fs";
import { join } from "node:path";
import { renderMarkdown } from "./markdown";

/**
 * In-app documentation (issue #598). Content lives as markdown in
 * `apps/web/content/docs` so it is reviewed and versioned with the code, and
 * is rendered through the same XSS-safe renderer used by contract notes.
 */
export interface DocMeta {
  slug: string;
  title: string;
  summary: string;
}

export const DOCS: DocMeta[] = [
  {
    slug: "getting-started",
    title: "Getting started",
    summary: "Run Sorolens locally and track your first contract.",
  },
  {
    slug: "concepts",
    title: "Concepts",
    summary: "Indexer, data model, API and watchdog explained.",
  },
];

// Next runs from apps/web, while a root-level test runner starts at the
// repository root. Accept both layouts.
const CONTENT_DIRS = [
  join(process.cwd(), "content", "docs"),
  join(process.cwd(), "apps", "web", "content", "docs"),
];

function readContent(file: string): string | undefined {
  for (const dir of CONTENT_DIRS) {
    try {
      return readFileSync(join(dir, file), "utf8");
    } catch {
      // Try the next candidate directory.
    }
  }
  return undefined;
}

export function listDocs(): DocMeta[] {
  return DOCS;
}

export function getDocMeta(slug: string): DocMeta | undefined {
  return DOCS.find((doc) => doc.slug === slug);
}

/** Reads one markdown file and renders it to safe HTML. */
export function renderDoc(slug: string): string | undefined {
  if (!getDocMeta(slug)) return undefined;
  const source = readContent(`${slug}.md`);
  return source === undefined ? undefined : renderMarkdown(source);
}

/** Reads a standalone markdown page that is not part of the docs sidebar. */
export function renderMarkdownPage(file: string): string | undefined {
  const source = readContent(`${file}.md`);
  return source === undefined ? undefined : renderMarkdown(source);
}
