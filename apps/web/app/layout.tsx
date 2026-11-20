import type { Metadata, Viewport } from "next";
import { headers } from "next/headers";
import { Suspense } from "react";
import "./globals.css";
import { NavigationProgress } from "@/components/NavigationProgress";
import { PlausibleAnalytics } from "@/components/PlausibleAnalytics";
import { SkipLink } from "@/components/SkipLink";
import { ThemeProvider } from "@/components/ThemeProvider";
import { NONCE_HEADER } from "@/lib/csp";
import { siteOrigin } from "@/lib/siteUrl";

export const metadata: Metadata = {
  metadataBase: new URL(siteOrigin()),
  manifest: "/manifest.json",
  title: {
    default: "Sorolens — Indexed Observability for Soroban",
    template: "%s | Sorolens",
  },
  description:
    "Indexed observability for Soroban smart contracts — events, invocations, storage, and anomaly detection on Stellar.",
  icons: {
    icon: [
      {
        url: "/favicon.ico",
        sizes: "16x16 32x32 48x48",
        type: "image/x-icon",
      },
      { url: "/favicon-16x16.png", type: "image/png", sizes: "16x16" },
      { url: "/favicon-32x32.png", type: "image/png", sizes: "32x32" },
      { url: "/favicon.svg", type: "image/svg+xml" },
    ],
    apple: [
      { url: "/apple-touch-icon.png", sizes: "180x180", type: "image/png" },
    ],
  },
  openGraph: {
    type: "website",
    siteName: "Sorolens",
    title: "Sorolens — Indexed Observability for Soroban",
    description:
      "Indexed observability for Soroban smart contracts — events, invocations, storage, and anomaly detection on Stellar.",
    url: "/",
    images: [
      {
        url: "/og-image.png",
        width: 1200,
        height: 630,
        alt: "Sorolens real-time Soroban contract monitoring dashboard",
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: "Sorolens — Indexed Observability for Soroban",
    description:
      "Indexed observability for Soroban smart contracts — events, invocations, storage, and anomaly detection on Stellar.",
    images: ["/og-image.png"],
  },
  alternates: {
    canonical: "/",
  },
};

export const viewport: Viewport = {
  themeColor: "#7c3aed",
};

export default async function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  // Reading the request also renders every page per request, which a nonce
  // requires: a statically generated page could not carry a fresh one.
  const nonce = (await headers()).get(NONCE_HEADER) ?? undefined;

  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <script
          nonce={nonce}
          dangerouslySetInnerHTML={{
            __html: `
              try {
                let theme = localStorage.getItem('theme') || 'system';
                let isDark = theme === 'dark' || (theme === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);
                if (isDark) document.documentElement.setAttribute('data-theme', 'dark');
                else document.documentElement.setAttribute('data-theme', 'light');
              } catch (e) {}
            `,
          }}
        />
        <PlausibleAnalytics nonce={nonce} />
      </head>
      <body className="min-h-screen antialiased">
        <SkipLink />
        <ThemeProvider>
          {/* useSearchParams inside NavigationProgress requires a Suspense boundary */}
          <Suspense fallback={null}>
            <NavigationProgress />
          </Suspense>
          {children}
        </ThemeProvider>
      </body>
    </html>
  );
}
