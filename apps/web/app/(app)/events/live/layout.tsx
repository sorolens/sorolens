import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Live Event Stream",
  description:
    "Real-time feed of Soroban contract events streamed over SSE, with a polling fallback, a contract filter and pause controls.",
  openGraph: {
    title: "Live Event Stream — Sorolens",
    description: "Real-time feed of Soroban contract events streamed over SSE.",
    url: "/events/live",
  },
  alternates: {
    canonical: "/events/live",
  },
};

export default function LiveEventsLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return children;
}
