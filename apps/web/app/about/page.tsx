import type { Metadata } from "next";
import { DocsShell } from "@/components/DocsShell";
import { renderMarkdownPage } from "@/lib/docs";

export const metadata: Metadata = {
  title: "About",
  description:
    "About Sorolens, the open-source observability dashboard for Soroban smart contracts on Stellar.",
  alternates: { canonical: "/about" },
};

export default function AboutPage() {
  const html = renderMarkdownPage("about");
  if (!html) {
    return (
      <DocsShell activeSlug="about">
        <p className="text-sm text-[var(--color-danger-text)]">
          About content is unavailable.
        </p>
      </DocsShell>
    );
  }

  return (
    <DocsShell activeSlug="about">
      <article
        className="docs-content"
        data-testid="about-content"
        dangerouslySetInnerHTML={{ __html: html }}
      />
    </DocsShell>
  );
}
