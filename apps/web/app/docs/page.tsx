import { readFile } from "node:fs/promises";
import { join } from "node:path";
import Image from "next/image";
import Link from "next/link";
import type { Metadata } from "next";
import { renderMarkdown } from "@/lib/markdown";

export const metadata: Metadata = {
  title: "Documentation",
  description:
    "Get started with Sorolens, learn the core concepts, and find the API reference and SDKs.",
};

export default async function DocsPage() {
  const markdown = await readFile(
    join(process.cwd(), "content", "docs", "getting-started.md"),
    "utf8"
  );

  return (
    <main className="min-h-screen">
      <header className="border-b border-[var(--color-border)]">
        <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-4 px-4 py-5 sm:px-6 lg:px-8">
          <Link
            href="/"
            className="flex items-center gap-2 text-xl font-bold tracking-tight"
          >
            <Image src="/logo.svg" alt="" width={28} height={28} aria-hidden />
            Sorolens
          </Link>
          <nav aria-label="Main navigation" className="flex gap-5 text-sm">
            <Link href="/contracts">Dashboard</Link>
            <Link href="/playground">Playground</Link>
            <Link href="/docs" aria-current="page">
              Docs
            </Link>
          </nav>
        </div>
      </header>
      <article
        className="markdown-content mx-auto max-w-4xl px-4 py-12 text-[var(--color-text-primary)] sm:px-6 lg:py-16"
        dangerouslySetInnerHTML={{ __html: renderMarkdown(markdown) }}
      />
      <footer className="border-t border-[var(--color-border)]">
        <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-4 px-4 py-6 text-sm text-[var(--color-text-secondary)] sm:px-6 lg:px-8">
          <span>Built for the Stellar developer community.</span>
          <nav aria-label="Footer navigation" className="flex flex-wrap gap-4">
            <Link href="/">Home</Link>
            <Link href="/docs">Docs</Link>
            <Link href="/playground">Playground</Link>
            <Link href="/compare">Compare</Link>
            <Link href="/watchdog/reports">Reports</Link>
            <a
              href="https://github.com/sorolens/sorolens"
              rel="noopener noreferrer"
            >
              GitHub
            </a>
          </nav>
        </div>
      </footer>
    </main>
  );
}
