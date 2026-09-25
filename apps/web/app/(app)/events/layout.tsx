import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Event Stream",
  description:
    "Real-time feed of Soroban contract events streamed via SSE. Filter by contract ID and watch decoded topics arrive live.",
  openGraph: {
    title: "Event Stream — Sorolens",
    description:
      "Real-time feed of Soroban contract events streamed via SSE.",
    url: "https://sorolens.dev/events",
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
