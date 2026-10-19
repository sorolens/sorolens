import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Settings",
  description:
    "Configure the Sorolens dashboard: theme, default network, notification preferences, and your personal API key.",
  openGraph: {
    title: "Settings — Sorolens",
    description: "Dashboard preferences and API key management for Sorolens.",
    url: "/settings",
  },
  alternates: {
    canonical: "/settings",
  },
};

export default function SettingsLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return children;
}
