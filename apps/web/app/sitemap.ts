import { MetadataRoute } from "next";

const baseUrl = process.env.NEXT_PUBLIC_SITE_URL ?? "http://localhost:3000";

const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL ??
  process.env.API_BASE_URL ??
  "http://localhost:3001";

export interface SitemapEntry {
  id: string;
  lastModified?: string | Date;
  createdAt?: string | Date;
  updatedAt?: string | Date;
}

const STATIC_ROUTES: Array<{
  path: string;
  changeFrequency: MetadataRoute.Sitemap[number]["changeFrequency"];
  priority: number;
}> = [
  { path: "", changeFrequency: "daily", priority: 1 },
  { path: "/contracts", changeFrequency: "daily", priority: 0.8 },
  { path: "/events", changeFrequency: "daily", priority: 0.8 },
  { path: "/events/live", changeFrequency: "hourly", priority: 0.7 },
  { path: "/invocations", changeFrequency: "daily", priority: 0.8 },
  { path: "/compare", changeFrequency: "weekly", priority: 0.6 },
  { path: "/playground", changeFrequency: "weekly", priority: 0.6 },
  { path: "/watchog", changeFrequency: "daily", priority: 0.8 },
  { path: "/watchdog/reports", changeFrequency: "daily", priority: 0.7 },
];

function parseTimestamp(entry: SitemapEntry): Date | undefined {
  const raw = entry.updatedAt ?? entry.lastModified ?? entry.createdAt;
  if (!raw) return undefined;
  const date = raw instanceof Date ? raw : new Date(raw);
  return Number.isNaN(date.getTime()) ? undefined : date;
}

export async function fetchTrackedContracts(): Promise<SitemapEntry[]> {
  try {
    const res = await fetch(`${API_BASE_URL}/api/contracts?tracked=true&limit=1000`, {
      next: { revalidate: 3600 },
      headers: { Accept: "application/json" },
    });

    if (!res.ok) return [];

    const json = (await res.json()) as unknown;
    const items = extractContractItems(json);
    return items;
  } catch {
    return [];
  }
}

function extractContractItems(json: unknown): SitemapEntry[] {
  if (!json || typeof json !== "object") return [];
  const candidate = json as {
    data?: unknown;
    items?: unknown;
    contracts?: unknown;
  };
  const list = candidate.data ?? candidate.items ?? candidate.contracts ?? json;
  if (!Array.isArray(list)) return [];
  return list
    .map((entry) => {
      if (!entry || typeof entry !== "object") return null;
      const record = entry as Record<string, unknown>;
      const id = record.id ?? record.contractId ?? record.address;
      if (typeof id !== "string" || id.length === 0) return null;
      return {
        id,
        lastModified: typeof record.lastModified === "string" ? record.lastModified : undefined,
        createdAt: typeof record.createdAt === "string" ? record.createdAt : undefined,
        updatedAt: typeof record.updatedAt === "string" ? record.updatedAt : undefined,
      } satisfies SitemapEntry;
    })
    .filter((entry): entry is SitemapEntry => entry !== null);
}

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const now = new Date();

  const staticEntries: MetadataRoute.Sitemap = STATIC_ROUTES.map((route) => ({
    url: `${baseUrl}${route.path}`,
    lastModified: now,
    changeFrequency: route.changeFrequency,
    priority: route.priority,
  }));

  const contracts = await fetchTrackedContracts();

  const contractEntries: MetadataRoute.Sitemap = contracts.map((contract) => ({
    url: `${baseUrl}/contracts/${encodeURIComponent(contract.id)}`,
    lastModified: parseTimestamp(contract) ?? now,
    changeFrequency: "daily",
    priority: 0.6,
  }));

  return [...staticEntries, ...contractEntries];
}
