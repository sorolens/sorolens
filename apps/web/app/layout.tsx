import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  metadataBase: new URL("https://sorolens.dev"),
  title: {
    default: "Sorolens — Indexed Observability for Soroban",
    template: "%s | Sorolens",
  },
  description:
    "Indexed observability for Soroban smart contracts — events, invocations, storage, and anomaly detection on Stellar.",
  icons: {
    icon: [{ url: "/favicon.svg", type: "image/svg+xml" }],
  },
  openGraph: {
    type: "website",
    siteName: "Sorolens",
    title: "Sorolens — Indexed Observability for Soroban",
    description:
      "Indexed observability for Soroban smart contracts — events, invocations, storage, and anomaly detection on Stellar.",
    url: "https://sorolens.dev",
  },
  alternates: {
    canonical: "https://sorolens.dev",
  },
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className="min-h-screen antialiased">{children}</body>
    </html>
  );
}
