import type { MetadataRoute } from "next";
import { absoluteUrl } from "@/lib/siteUrl";
import { getSitemapContracts } from "@/lib/serverData";

type PublicRoute = {
  path: string;
  changeFrequency: NonNullable<
    MetadataRoute.Sitemap[number]["changeFrequency"]
  >;
  priority: number;
};

/**
 * Public, non-personal routes.
 *
 * Personal surfaces (`/dashboard`, `/settings`, `/watchlist`, `/groups`,
 * `/rules`) and transactional ones (`/contracts/new`, `/search`,
 * `/offline-alerts`) stay out: a crawler has no session, so the only version of
 * those pages it can ever fetch is an empty one. Every path here is indexable
 * content that renders the same for every visitor.
 */
const PUBLIC_ROUTES: readonly PublicRoute[] = [
  { path: "/", changeFrequency: "hourly", priority: 1 },
  { path: "/contracts", changeFrequency: "daily", priority: 0.9 },
  { path: "/events", changeFrequency: "hourly", priority: 0.8 },
  { path: "/events/live", changeFrequency: "hourly", priority: 0.7 },
  { path: "/invocations", changeFrequency: "hourly", priority: 0.8 },
  { path: "/compare", changeFrequency: "daily", priority: 0.7 },
  { path: "/playground", changeFrequency: "weekly", priority: 0.6 },
  { path: "/watchdog", changeFrequency: "daily", priority: 0.8 },
  { path: "/watchdog/reports", changeFrequency: "daily", priority: 0.7 },
  { path: "/live", changeFrequency: "hourly", priority: 0.6 },
  { path: "/status", changeFrequency: "daily", priority: 0.5 },
  { path: "/docs", changeFrequency: "weekly", priority: 0.7 },
  { path: "/docs/getting-started", changeFrequency: "weekly", priority: 0.6 },
  { path: "/docs/concepts", changeFrequency: "weekly", priority: 0.6 },
  { path: "/about", changeFrequency: "monthly", priority: 0.5 },
  { path: "/integrations", changeFrequency: "weekly", priority: 0.5 },
];

/**
 * Stamped on entries the API has no timestamp for. It is fixed when the process
 * starts — the build time for a prerendered sitemap — instead of `new Date()`
 * per request, so a cached entry does not claim to have changed every time it
 * is fetched.
 */
const PROCESS_STARTED_AT = new Date();

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const statics: MetadataRoute.Sitemap = PUBLIC_ROUTES.map((route) => ({
    url: absoluteUrl(route.path),
    lastModified: PROCESS_STARTED_AT,
    changeFrequency: route.changeFrequency,
    priority: route.priority,
  }));

  const contracts = await getSitemapContracts();
  const dynamic: MetadataRoute.Sitemap = contracts.map((contract) => ({
    url: absoluteUrl(`/contracts/${encodeURIComponent(contract.id)}`),
    // `last_activity_at` is null until a contract's first event is indexed, so
    // the date it was tracked is the fallback.
    lastModified: new Date(contract.last_activity_at ?? contract.added_at),
    changeFrequency: "hourly",
    priority: 0.6,
  }));

  return [...statics, ...dynamic];
}
