import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Watchdog",
  description:
    "On-chain anomaly detection for Soroban contracts. View monitored contracts, health scores, and triggered alerts powered by the Watchdog smart contract.",
  openGraph: {
    title: "Watchdog — Sorolens",
    description: "On-chain anomaly detection dashboard for Soroban contracts.",
    url: "https://sorolens.dev/watchdog",
  },
  alternates: {
    canonical: "/watchdog",
  },
};

export default function WatchdogLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return children;
}
