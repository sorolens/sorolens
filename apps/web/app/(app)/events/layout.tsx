import type { Metadata } from "next";


export const metadata: Metadata = {
  title: "Events",
  description:
    "Explore and filter decoded Soroban contract events indexed by Sorolens. Search by contract ID and inspect topics and event data.",
  openGraph: {
    title: "Events — Sorolens",
    description:
      "Explore and filter decoded Soroban contract events indexed by Sorolens.",
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
