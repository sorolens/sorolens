import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Watchlist",
  description:
    "Your personal watchlist of starred Soroban contracts. Quickly access the contracts you care about most.",
  openGraph: {
    title: "Watchlist — Sorolens",
    description: "Personal watchlist of starred Soroban contracts on Sorolens.",
    url: "https://sorolens.dev/watchlist",
  },
  alternates: {
    canonical: "/watchlist",
  },
};

export default function WatchlistLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return children;
}
