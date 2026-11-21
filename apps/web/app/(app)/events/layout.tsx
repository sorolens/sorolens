import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Contract Events",
  description:
    "Search stored Soroban contract events across every tracked contract. Filter by contract, event type and ledger range, and inspect decoded topics.",
  openGraph: {
    title: "Contract Events — Sorolens",
    description:
      "Search stored Soroban contract events across every tracked contract, filterable by contract, type and ledger range.",
    url: "/events",
  },
  alternates: {
    canonical: "/events",
  },
};

export default function EventsLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return children;
}
