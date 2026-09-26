import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Compare Contracts",
  description:
    "Side-by-side comparison of Soroban contract metrics — event volume, invocation costs, health scores, and 7-day trends.",
  openGraph: {
    title: "Compare Contracts — Sorolens",
    description: "Side-by-side comparison of Soroban contract metrics.",
    url: "https://sorolens.dev/compare",
  },
  alternates: {
    canonical: "/compare",
  },
};

export default function CompareLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return children;
}
