import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { DocsShell } from "@/components/DocsShell";
import { DOCS, getDocMeta, renderDoc } from "@/lib/docs";

interface Props {
  params: Promise<{ slug: string }>;
}

export function generateStaticParams() {
  return DOCS.map((doc) => ({ slug: doc.slug }));
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params;
  const meta = getDocMeta(slug);
  if (!meta) return { title: "Documentation" };
  return {
    title: meta.title,
    description: meta.summary,
    alternates: { canonical: `/docs/${meta.slug}` },
  };
}

export default async function DocPage({ params }: Props) {
  const { slug } = await params;
  const html = renderDoc(slug);
  if (!html) notFound();
  const meta = getDocMeta(slug)!;

  return (
    <DocsShell activeSlug={slug}>
      <article
        className="docs-content"
        data-testid="docs-content"
        dangerouslySetInnerHTML={{ __html: html }}
      />
      <p className="mt-8 text-sm text-[var(--color-text-secondary)]">
        {meta.title} is maintained in{" "}
        <a
          href={`https://github.com/sorolens/sorolens/blob/main/apps/web/content/docs/${slug}.md`}
          rel="noopener noreferrer"
          className="text-[var(--color-accent)] underline underline-offset-2"
        >
          apps/web/content/docs/{slug}.md
        </a>
        .
      </p>
    </DocsShell>
  );
}
