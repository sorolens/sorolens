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
import { NetworkProvider } from "@/lib/network";
import { NetworkSelector } from "@/components/NetworkSelector";
import { Breadcrumbs } from "@/components/Breadcrumbs";
import { CmdkSearch } from "@/components/CmdkSearch";
import { LastUpdated } from "@/components/LastUpdated";
import { OfflineBanner } from "@/components/OfflineAlert";
import { PushSubscribeButton } from "@/components/PushSubscribeButton";
import { ThemeToggle } from "@/components/ThemeToggle";
import { useOfflineAlertQueue } from "@/hooks/useOfflineAlertQueue";

function AppLayoutInner({ children }: { children: React.ReactNode }) {
  const { pending, isOffline } = useOfflineAlertQueue();

  return (
    <>
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
            <nav className="flex gap-4 text-sm">
              <Link
                href="/live"
                className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
              >
                Live
              </Link>
              <Link
                href="/contracts"
                className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
              >
                Contracts
              </Link>
              <Link
                href="/events"
                className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
              >
                Events
              </Link>
              <Link
                href="/watchdog"
                className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
              >
                Watchdog
              </Link>
              <Link
                href="/playground"
                className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
              >
                Playground
              </Link>
              <Link
                href="/settings"
                className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
              >
                Settings
              </Link>
            </nav>
            <NetworkSelector />
            <PushSubscribeButton />
            <ThemeToggle />
          </div>
        </header>
        <Breadcrumbs />
        <main>{children}</main>
        <footer className="mt-12 border-t border-[var(--color-border)] pt-6 text-sm text-[var(--color-text-secondary)]">
          Built for the Stellar developer community.
          <LastUpdated />
        </footer>
      </div>
    </>
  );
}

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <NetworkProvider>
      <AppLayoutInner>{children}</AppLayoutInner>
    </NetworkProvider>
  );
}
