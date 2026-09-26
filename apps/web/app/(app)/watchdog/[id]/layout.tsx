import type { Metadata } from "next";

type Props = {
  params: Promise<{ id: string }>;
};

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params;
  const shortId = id.length > 12 ? `${id.slice(0, 6)}…${id.slice(-4)}` : id;

  return {
    title: `Watchdog · ${shortId}`,
    description: `Health checks, alerts, and anomaly detection history for monitored Soroban contract ${id}.`,
    openGraph: {
      title: `Watchdog · ${shortId} — Sorolens`,
      description: `Health checks and alert history for Soroban contract ${id}.`,
      url: `https://sorolens.dev/watchdog/${id}`,
    },
    alternates: {
      canonical: `/watchdog/${id}`,
    },
  };
}

export default function WatchdogDetailLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return children;
}
