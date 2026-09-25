import type { Metadata } from "next";
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
    icon: [{ url: "/favicon.svg", type: "image/svg+xml" }],
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
