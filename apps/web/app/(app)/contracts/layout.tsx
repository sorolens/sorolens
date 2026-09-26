import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Contracts",
  description:
    "Browse, search, and track Soroban smart contracts indexed by Sorolens. View event counts, invocation stats, and storage health at a glance.",
  openGraph: {
    title: "Contracts — Sorolens",
    description:
      "Browse, search, and track Soroban smart contracts indexed by Sorolens.",
    url: "https://sorolens.dev/contracts",
  },
  alternates: {
    canonical: "/contracts",
  },
};

export default function ContractsLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return children;
}
