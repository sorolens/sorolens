import type { Metadata } from "next";
import Link from "next/link";

type Props = {
  params: Promise<{ id: string }>;
};

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params;
  const shortId = id.length > 12 ? `${id.slice(0, 6)}…${id.slice(-4)}` : id;

  return {
    title: `Contract ${shortId}`,
    description: `Events, invocations, storage health, and anomaly detection for Soroban contract ${id}.`,
    openGraph: {
      title: `Contract ${shortId} — Sorolens`,
      description: `Detailed observability dashboard for Soroban contract ${id}.`,
      url: `https://sorolens.dev/contracts/${id}`,
    },
    alternates: {
      canonical: `/contracts/${id}`,
    },
  };
}

export default function ContractDetailLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-4">
      <nav
        aria-label="Contract exports"
        className="flex flex-wrap items-center gap-3 text-sm"
      >
        <Link
          href={`/contracts/${id}/snapshot.json`}
          download
          className="rounded-md border border-neutral-300 px-3 py-1.5 font-medium hover:bg-neutral-100 dark:border-neutral-700 dark:hover:bg-neutral-800"
        >
          Download snapshot.json
        </Link>
        <Link
          href={`/contracts/${id}/summary`}
          className="rounded-md border border-neutral-300 px-3 py-1.5 font-medium hover:bg-neutral-100 dark:border-neutral-700 dark:hover:bg-neutral-800"
        >
          View summary
        </Link>
      </nav>
      {children}
    </div>
  );
}
