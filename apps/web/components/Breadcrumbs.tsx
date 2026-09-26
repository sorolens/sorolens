"use client";

import Link from "next/link";
import { useParams, usePathname } from "next/navigation";

/**
 * Breadcrumb trail for dashboard pages.
 *
 * Builds "Home > Contracts > <id>" style navigation from the current route.
 * Every crumb, including the current page, is a link. Dynamic route params
 * (e.g. a contract id) are truncated so long identifiers do not overflow the
 * trail; the full value stays visible in the link's href and title.
 */

const SECTION_LABELS: Record<string, string> = {
  compare: "Compare",
  contracts: "Contracts",
  events: "Events",
  playground: "Playground",
  watchdog: "Watchdog",
  watchlist: "Watchlist",
};

const ID_HEAD_CHARS = 6;
const ID_TAIL_CHARS = 4;

function truncateId(value: string): string {
  if (value.length <= ID_HEAD_CHARS + ID_TAIL_CHARS + 3) {
    return value;
  }

  return `${value.slice(0, ID_HEAD_CHARS)}...${value.slice(-ID_TAIL_CHARS)}`;
}

function safeDecode(segment: string): string {
  try {
    return decodeURIComponent(segment);
  } catch {
    return segment;
  }
}

const LINK_CLASS =
  "text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]";

const CURRENT_CLASS = "text-[var(--color-text-primary)]";

export function Breadcrumbs() {
  const pathname = usePathname();
  const params = useParams();

  const segments = (pathname ?? "/").split("/").filter(Boolean);
  if (segments.length === 0) {
    return null;
  }

  const paramValues = new Set(
    Object.values(params ?? {}).flatMap((value) =>
      (Array.isArray(value) ? value : [value]).map(String)
    )
  );

  const crumbs = segments.map((segment, index) => {
    const decoded = safeDecode(segment);
    const isParamValue = paramValues.has(decoded);
    const label = isParamValue
      ? truncateId(decoded)
      : (SECTION_LABELS[decoded] ?? decoded);
    const href = `/${segments.slice(0, index + 1).join("/")}`;
    return { href, label, decoded };
  });

  return (
    <nav aria-label="Breadcrumb" className="mb-6">
      <ol className="flex flex-wrap items-center gap-1.5 text-sm">
        <li className="flex items-center gap-1.5">
          <Link href="/" className={LINK_CLASS}>
            Home
          </Link>
        </li>
        {crumbs.map((crumb, index) => {
          const isCurrent = index === crumbs.length - 1;
          return (
            <li key={crumb.href} className="flex items-center gap-1.5">
              <span aria-hidden="true" className={LINK_CLASS}>
                /
              </span>
              <Link
                href={crumb.href}
                title={crumb.decoded}
                aria-current={isCurrent ? "page" : undefined}
                className={isCurrent ? CURRENT_CLASS : LINK_CLASS}
              >
                {crumb.label}
              </Link>
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
