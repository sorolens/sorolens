import type { Metadata } from "next";
import { ContractSummaryCard } from "@/components/contracts/contract-summary-card";
import { SnapshotExportLinks } from "@/components/contracts/snapshot-export-links";

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
  params,
}: {
  children: React.ReactNode;
  params: Promise<{ id: string }>;
}) {
  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <ContractSummaryCard contractIdPromise={params} />
        <SnapshotExportLinks contractIdPromise={params} />
      </div>
      {children}
    </div>
  );
}
