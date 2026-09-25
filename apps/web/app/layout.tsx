import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  metadataBase: new URL("https://sorolens.xyz"),
  title: "Sorolens: Real-time Soroban Contract Monitoring",
  description:
    "On-chain health checks and observability for Soroban smart contracts on Stellar.",
  openGraph: {
    title: "Sorolens: Real-time Soroban Contract Monitoring",
    description:
      "On-chain health checks and observability for Soroban smart contracts on Stellar.",
    url: "https://sorolens.xyz",
    siteName: "Sorolens",
    images: [
      {
        url: "/og-image.png",
        width: 1200,
        height: 630,
        alt: "Sorolens real-time Soroban contract monitoring dashboard",
      },
    ],
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "Sorolens: Real-time Soroban Contract Monitoring",
    description:
      "On-chain health checks and observability for Soroban smart contracts on Stellar.",
    images: ["/og-image.png"],
  },
  icons: {
    icon: [
      {
        url: "/favicon.ico",
        sizes: "16x16 32x32 48x48",
        type: "image/x-icon",
      },
      { url: "/favicon.svg", type: "image/svg+xml" },
    ],
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
