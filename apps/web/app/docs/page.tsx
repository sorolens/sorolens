import type { Metadata } from "next";
import Link from "next/link";
import { DocsShell } from "@/components/DocsShell";
import { listDocs } from "@/lib/docs";

export const metadata: Metadata = {
  title: "Documentation",
  description:
    "Get started with Sorolens, learn the indexing concepts, and find the API reference and SDKs.",
  alternates: { canonical: "/docs" },
};

export default function DocsIndexPage() {
  return (
    <DocsShell>
      <div className="docs-content">
        <h1>Documentation</h1>
        <p>
          Everything you need to run Sorolens, track a Soroban contract, and
          query its indexed data. The source lives in the repository, so these
          pages are versioned with the code they describe.
        </p>
      </div>

      <div className="mt-8 grid gap-4 sm:grid-cols-2">
        {listDocs().map((doc) => (
          <Link
            key={doc.slug}
            href={`/docs/${doc.slug}`}
            className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-5 transition-colors hover:border-[var(--color-border-hover)]"
          >
            <h2 className="text-lg font-semibold">{doc.title}</h2>
            <p className="mt-1 text-sm text-[var(--color-text-secondary)]">
              {doc.summary}
            </p>
          </Link>
        ))}
      </div>

      <section className="mt-10">
        <h2 className="text-lg font-semibold">Reference</h2>
        <ul className="mt-3 space-y-2 text-sm">
          <li>
            <Link
              href="/playground"
              className="text-[var(--color-accent)] underline underline-offset-2"
            >
              API playground
            </Link>{" "}
            — browse every documented endpoint.
          </li>
          <li>
            <a
              href="https://github.com/sorolens/sorolens/blob/main/docs/openapi.yaml"
              rel="noopener noreferrer"
              className="text-[var(--color-accent)] underline underline-offset-2"
            >
              OpenAPI specification
            </a>{" "}
            — the source of truth for the REST surface.
          </li>
          <li>
            <a
              href="https://github.com/sorolens/sorolens/tree/main/packages"
              rel="noopener noreferrer"
              className="text-[var(--color-accent)] underline underline-offset-2"
            >
              SDKs and clients
            </a>{" "}
            — Go, TypeScript and Python packages.
          </li>
          <li>
            <a
              href="https://github.com/sorolens/sorolens/blob/main/ARCHITECTURE.md"
              rel="noopener noreferrer"
              className="text-[var(--color-accent)] underline underline-offset-2"
            >
              Architecture overview
            </a>{" "}
            — how the indexer, API and dashboard fit together.
          </li>
        </ul>
      </section>
    </DocsShell>
  );
}
