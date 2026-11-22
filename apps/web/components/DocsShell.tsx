import Link from "next/link";
import { listDocs } from "@/lib/docs";

/**
 * Shared chrome for the in-product documentation and about pages (issue #598).
 * Keeps the docs reachable from every docs view without changing the
 * dashboard app shell.
 */
export function DocsShell({
  children,
  activeSlug,
}: {
  children: React.ReactNode;
  activeSlug?: string;
}) {
  return (
    <div className="mx-auto max-w-5xl px-4 py-6 sm:px-6 lg:px-8">
      <header className="mb-8 flex flex-wrap items-center justify-between gap-4">
        <Link
          href="/"
          className="flex items-center gap-2 text-2xl font-bold tracking-tight"
        >
          <img src="/logo.svg" alt="" className="h-8 w-8" aria-hidden />
          Sorolens
        </Link>
        <nav
          aria-label="Documentation"
          className="flex flex-wrap gap-4 text-sm"
        >
          <Link
            href="/docs"
            className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
          >
            Docs
          </Link>
          <Link
            href="/about"
            className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
          >
            About
          </Link>
          <Link
            href="/contracts"
            className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
          >
            Dashboard
          </Link>
          <Link
            href="/playground"
            className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
          >
            API playground
          </Link>
          <a
            href="https://github.com/sorolens/sorolens"
            rel="noopener noreferrer"
            className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
          >
            GitHub
          </a>
        </nav>
      </header>

      <div className="grid gap-8 md:grid-cols-[12rem_1fr]">
        <aside>
          <nav aria-label="Documentation sections" className="text-sm">
            <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-[var(--color-text-muted)]">
              Documentation
            </p>
            <ul className="space-y-1">
              {listDocs().map((doc) => (
                <li key={doc.slug}>
                  <Link
                    href={`/docs/${doc.slug}`}
                    aria-current={activeSlug === doc.slug ? "page" : undefined}
                    className={`block rounded-md px-2 py-1 transition-colors ${
                      activeSlug === doc.slug
                        ? "bg-[var(--color-accent-bg)] text-[var(--color-accent-text)]"
                        : "text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-hover)] hover:text-[var(--color-text-primary)]"
                    }`}
                  >
                    {doc.title}
                  </Link>
                </li>
              ))}
              <li>
                <Link
                  href="/about"
                  aria-current={activeSlug === "about" ? "page" : undefined}
                  className={`block rounded-md px-2 py-1 transition-colors ${
                    activeSlug === "about"
                      ? "bg-[var(--color-accent-bg)] text-[var(--color-accent-text)]"
                      : "text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-hover)] hover:text-[var(--color-text-primary)]"
                  }`}
                >
                  About
                </Link>
              </li>
            </ul>
          </nav>
        </aside>

        <main id="content" className="min-w-0">
          {children}
        </main>
      </div>

      <footer className="mt-12 border-t border-[var(--color-border)] pt-6 text-sm text-[var(--color-text-secondary)]">
        <div className="flex flex-wrap gap-4">
          <Link href="/docs" className="hover:text-[var(--color-text-primary)]">
            Documentation
          </Link>
          <Link
            href="/about"
            className="hover:text-[var(--color-text-primary)]"
          >
            About
          </Link>
          <a
            href="https://github.com/sorolens/sorolens/blob/main/CONTRIBUTING.md"
            rel="noopener noreferrer"
            className="hover:text-[var(--color-text-primary)]"
          >
            Contribute
          </a>
          <a
            href="https://discord.gg/D9jATUezYX"
            rel="noopener noreferrer"
            className="hover:text-[var(--color-text-primary)]"
          >
            Discord
          </a>
        </div>
      </footer>
    </div>
  );
}
