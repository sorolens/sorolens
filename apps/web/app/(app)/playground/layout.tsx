import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "API Playground",
  description:
    "Interactive API explorer for the Sorolens REST API. Select an endpoint, fill parameters, and execute requests directly from the browser.",
  openGraph: {
    title: "API Playground — Sorolens",
    description: "Interactive API explorer for the Sorolens REST API.",
    url: "https://sorolens.dev/playground",
  },
  alternates: {
    canonical: "/playground",
  },
};

export default function PlaygroundLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return children;
}
