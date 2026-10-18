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

const NAV_LINKS = [
  { href: "/live", labelKey: "nav.live" },
  { href: "/contracts", labelKey: "nav.contracts" },
  { href: "/events", labelKey: "nav.events" },
  { href: "/watchdog", labelKey: "nav.watchdog" },
  { href: "/watchdog/reports", label: "Reports" },
  { href: "/groups", labelKey: "nav.groups" },
  { href: "/rules", labelKey: "nav.rules" },
  { href: "/playground", labelKey: "nav.playground" },
  { href: "/settings", labelKey: "nav.settings" },
] as const;

function isActive(pathname: string, href: string): boolean {
  if (href === "/") return pathname === "/";
  return pathname === href || pathname.startsWith(`${href}/`);
}

function AppLayoutInner({ children }: { children: React.ReactNode }) {
  const { pending, isOffline } = useOfflineAlertQueue();
  const { t } = useLocale();
  const pathname = usePathname() ?? "";

  return (
    <>
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:absolute focus:left-4 focus:top-4 focus:z-50 focus:rounded-md focus:bg-[--color-bg-elevated] focus:px-4 focus:py-2 focus:text-sm focus:font-medium focus:text-[--color-text-primary] focus:ring-2 focus:ring-[--color-accent]"
      >
        Skip to content
      </a>
      <OfflineBanner isOffline={isOffline} pendingCount={pending.length} />
      <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
        <header className="mb-8 flex flex-wrap items-center justify-between gap-4">
          <Link
            href="/"
            className="flex items-center gap-2 text-2xl font-bold tracking-tight"
          >
            <img src="/logo.svg" alt="" className="h-8 w-8" aria-hidden />
            Sorolens
          </Link>
          <div className="flex flex-wrap items-center gap-4">
            <nav aria-label="Main" className="flex gap-4 text-sm">
              {NAV_LINKS.map(({ href, labelKey, label }) => {
                const active = isActive(pathname, href);
                return (
                  <Link
                    key={href}
                    href={href}
                    aria-current={active ? "page" : undefined}
                    className={`${
                      active
                        ? "text-[--color-text-primary] font-medium"
                        : "text-[--color-text-secondary]"
                    } transition-colors hover:text-[--color-text-primary]`}
                  >
                    {labelKey ? t(labelKey) : label}
                  </Link>
                );
              })}
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
        <footer className="mt-12 border-t border-[--color-border] pt-6 text-sm text-[--color-text-secondary]">
          {t("shell.community")}
          <Link
            href="/status"
            className="ml-3 hover:text-[--color-text-primary]"
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
