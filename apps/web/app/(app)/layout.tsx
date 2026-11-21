"use client";

/**
 * App-shell layout for all dashboard routes (/(app)/...).
 *
 * - Mounts useOfflineAlertQueue so alerts can be captured while offline.
 * - Renders OfflineBanner at the top when offline.
 * - Renders PushSubscribeButton in the header so the user can opt in to
 *   Web Push at any time.
 */
import Link from "next/link";
import Image from "next/image";
import { usePathname } from "next/navigation";
import { SWRConfig } from "swr";
import { NetworkProvider } from "@/lib/network";
import { swrConfig } from "@/lib/swr";
import { NetworkSelector } from "@/components/NetworkSelector";
import { Breadcrumbs } from "@/components/Breadcrumbs";
import { CmdkSearch } from "@/components/CmdkSearch";
import { LastUpdated } from "@/components/LastUpdated";
import { OfflineBanner } from "@/components/OfflineAlert";
import { PushSubscribeButton } from "@/components/PushSubscribeButton";
import { ThemeToggle } from "@/components/ThemeToggle";
import { LocalePreferences } from "@/components/LocalePreferences";
import { LocaleProvider, useLocale } from "@/lib/i18n";
import { useOfflineAlertQueue } from "@/hooks/useOfflineAlertQueue";
import { AppShortcuts } from "@/components/AppShortcuts";

const NAV_HREFS = [
  "/dashboard",
  "/live",
  "/contracts",
  "/events",
  "/invocations",
  "/compare",
  "/watchlist",
  "/watchdog",
  "/watchdog/reports",
  "/integrations",
  "/groups",
  "/rules",
  "/playground",
  "/settings",
] as const;

/**
 * The active nav entry is the longest href matching the current path, so
 * /watchdog/reports highlights Reports alone rather than both Reports and
 * Watchdog.
 */
function activeHref(pathname: string | null): string | null {
  if (!pathname) return null;
  let active: string | null = null;
  for (const href of NAV_HREFS) {
    if (pathname === href || pathname.startsWith(`${href}/`)) {
      if (!active || href.length > active.length) active = href;
    }
  }
  return active;
}

function AppLayoutInner({ children }: { children: React.ReactNode }) {
  const { pending, isOffline } = useOfflineAlertQueue();
  const { t } = useLocale();
  const active = activeHref(usePathname());

  return (
    <>
      <OfflineBanner isOffline={isOffline} pendingCount={pending.length} />
      <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
        <header className="mb-8 flex flex-wrap items-center justify-between gap-4">
          <Link
            href="/"
            className="flex items-center gap-2 text-2xl font-bold tracking-tight"
          >
            <Image
              src="/logo.svg"
              alt=""
              width={32}
              height={32}
              priority
              className="h-8 w-8"
              aria-hidden
            />
            Sorolens
          </Link>
          <div className="flex flex-wrap items-center gap-4">
            <nav
              aria-label="Primary"
              className="flex flex-wrap gap-x-4 gap-y-1 text-sm"
            >
              <Link
                href="/dashboard"
                aria-current={active === "/dashboard" ? "page" : undefined}
                className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
              >
                Dashboard
              </Link>
              <Link
                href="/live"
                aria-current={active === "/live" ? "page" : undefined}
                className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
              >
                {t("nav.live")}
              </Link>
              <Link
                href="/contracts"
                aria-current={active === "/contracts" ? "page" : undefined}
                className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
              >
                {t("nav.contracts")}
              </Link>
              <Link
                href="/events"
                aria-current={active === "/events" ? "page" : undefined}
                className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
              >
                {t("nav.events")}
              </Link>
              <Link
                href="/invocations"
                aria-current={active === "/invocations" ? "page" : undefined}
                className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
              >
                {t("nav.invocations")}
              </Link>
              <Link
                href="/compare"
                aria-current={active === "/compare" ? "page" : undefined}
                className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
              >
                {t("nav.compare")}
              </Link>
              <Link
                href="/watchlist"
                aria-current={active === "/watchlist" ? "page" : undefined}
                className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
              >
                {t("nav.watchlist")}
              </Link>
              <Link
                href="/watchdog"
                aria-current={active === "/watchdog" ? "page" : undefined}
                className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
              >
                {t("nav.watchdog")}
              </Link>
              <Link
                href="/watchdog/reports"
                aria-current={
                  active === "/watchdog/reports" ? "page" : undefined
                }
                className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
              >
                Reports
              </Link>
              <Link
                href="/integrations"
                aria-current={active === "/integrations" ? "page" : undefined}
                className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
              >
                Integrations
              </Link>
              <Link
                href="/groups"
                aria-current={active === "/groups" ? "page" : undefined}
                className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
              >
                {t("nav.groups")}
              </Link>
              <Link
                href="/rules"
                aria-current={active === "/rules" ? "page" : undefined}
                className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
              >
                {t("nav.rules")}
              </Link>
              <Link
                href="/playground"
                aria-current={active === "/playground" ? "page" : undefined}
                className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
              >
                {t("nav.playground")}
              </Link>
              <Link
                href="/settings"
                aria-current={active === "/settings" ? "page" : undefined}
                className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
              >
                {t("nav.settings")}
              </Link>
            </nav>
            <LocalePreferences />
            <NetworkSelector />
            <CmdkSearch />
            <PushSubscribeButton />
            <ThemeToggle />
          </div>
        </header>
        <Breadcrumbs />
        <main id="main" tabIndex={-1}>
          {children}
        </main>
        <AppShortcuts />
        <footer className="mt-12 border-t border-[var(--color-border)] pt-6 text-sm text-[var(--color-text-secondary)]">
          {t("shell.community")}
          <Link
            href="/status"
            className="ml-3 hover:text-[var(--color-text-primary)]"
          >
            System status
          </Link>
          <LastUpdated />
        </footer>
      </div>
    </>
  );
}

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <LocaleProvider>
      <SWRConfig value={swrConfig}>
        <NetworkProvider>
          <AppLayoutInner>{children}</AppLayoutInner>
        </NetworkProvider>
      </SWRConfig>
    </LocaleProvider>
  );
}
