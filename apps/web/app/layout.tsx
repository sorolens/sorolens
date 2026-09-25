import type { Metadata, Viewport } from "next";
import { Suspense } from "react";
import "./globals.css";
import { NavigationProgress } from "@/components/NavigationProgress";
import { ThemeProvider } from "@/components/ThemeProvider";

export const metadata: Metadata = {
  metadataBase: new URL("https://sorolens.dev"),
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
    url: "https://sorolens.dev",
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
    canonical: "https://sorolens.dev",
  },
};

export const viewport: Viewport = {
  themeColor: "#7c3aed",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <script
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
      </head>
      <body className="min-h-screen antialiased">
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
