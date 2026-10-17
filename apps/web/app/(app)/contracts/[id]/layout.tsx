import type { Metadata } from "next";
import { getContractForMetadata } from "@/lib/serverData";

type Props = {
  params: Promise<{ id: string }>;
};

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params;
  const shortId = id.length > 12 ? `${id.slice(0, 6)}…${id.slice(-4)}` : id;
  const contract = await getContractForMetadata(id);
  const label = contract?.label?.trim() || `Contract ${shortId}`;
  const network = contract?.network;
  const title = network ? `${label} (${network})` : label;
  const description = network
    ? `Events, invocations, storage health, and anomaly detection for ${label} on ${network}.`
    : `Events, invocations, storage health, and anomaly detection for Soroban contract ${id}.`;

  return {
    title,
    description,
    openGraph: {
      title: `${title} — Sorolens`,
      description,
      url: `/contracts/${id}`,
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
  return children;
}
