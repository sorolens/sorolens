import type { Metadata } from "next";
import "./globals.css";

const title = "Sorolens: Real-time Soroban Contract Monitoring";
const description =
  "On-chain health checks and observability for Soroban smart contracts on Stellar.";
const socialImage = {
  url: "/og-image.png",
  width: 1200,
  height: 630,
  alt: title,
};

export const metadata: Metadata = {
  metadataBase: new URL("https://sorolens-web-iota.vercel.app"),
  title,
  description,
  openGraph: {
    type: "website",
    siteName: "Sorolens",
    title,
    description,
    url: "/",
    images: [socialImage],
  },
  twitter: {
    card: "summary_large_image",
    title,
    description,
    images: [socialImage],
  },
  icons: {
    icon: [{ url: "/favicon.svg", type: "image/svg+xml" }],
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
