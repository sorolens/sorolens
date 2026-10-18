import type { MetadataRoute } from "next";

const BASE_URL =
  process.env.NEXT_PUBLIC_SITE_URL || "http://localhost:3000";

interface TrackedContract {
  id: string;
  updatedAt?: string | null;
  createdAt?: string | null;
}

const STATIC_ROUTES: Array<{
  path: string;
  changeFrequency: MetadataRoute["changeFrequency"];
  priority: number;
}> = [
  { path: "", changeFrequency: "daily", priority: 1 },
  { path: "/contracts", changeFrequency: "daily", priority: 0.8 },
  { path: "/events", changeFrequency: "hourly", priority: 0.8 },
  { path: "/events/live", changeFrequency: "hourly", priority: 0.7 },
  { path: "/invocations", changeFrequency: "daily", priority: 0.7 },
  { path: "/compare", changeFrequency: "weekly", priority: 0.6 },
  { path: "/playground", changeFrequency: "weekly", priority: 0.6 },
  { path: "/watchrdog", changeFrequency: "daily", priority: 0.8 },
  { path: "/watchdog/reports", changeFrequency: "daily", priority: 0.7 },
];

function toDate(value?: string | null): Date | undefined {
  if (!value) return undefined;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? undefined : date;
}

async function fetchTrackedContracts(): Promise<TrackedContract[]> {
  const apiBaseUrl =
    process.env.NEXT_PUBLIC_API_URL ||
    process.env.API_URL ||
    "http://localhost:4000";

  try {
    const res = await fetch(`${apiBaseUrl}/contracts`, {
      headers: { Accept: "application/json" },
      next: { revalidate: 3600 },
    });

    if (!res.ok) return [];

    const json = (await res.json()) as unknown;
    const items = Array.isArray(json)
      ? json
      : Array.isArray((json as { data?: unknown })?.data)
        ? ((json as { data: unknown }).data as unknown[])
        : [];

    return items
      .map((item) => item as Record<string, unknown>)
      .filter((item) => typeof item?.id === "string")
      .map((item) => ({
        id: item.id as string,
        updatedAt: (item.updatedAt as string | undefined) ?? null,
        createdAt: (item.createdAt as string | undefined) ?? null,
      }));
  } catch {
    return [];
  }
}

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const now = new Date();

  const staticEntries: MetadataRoute.Sitemap = STATIC_ROUTES.map(
    ({ path, changeFrequency, priority }) => ({
      url: `${BASE_URL}${path}`,
      lastModified: now,
      changeFrequency,
      priority,
    }),
  );

  const contracts = await fetchTrackedContracts();

  const contractEntries: MetadataRoute.Sitemap = contracts.map(
    (contract) => ({
      url: `${BASE_URL}/contracts/${encodeURIComponent(contract.id)}`,
      lastModified:
        toDate(contract.updatedAt) ??
        toDate(contract.createdAt) ?? now,
      changeFrequency: "daily",
      priority: 0.6,
    }),
  );

  return [...staticEntries, ...contractEntries];
}
