import Link from "next/link";
import { NetworkProvider } from "@/lib/network";
import { NetworkSelector } from "@/components/NetworkSelector";
import { LastUpdated } from "@/components/LastUpdated";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <NetworkProvider>
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
                href="/contracts"
                className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
              >
                Contracts
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
            </nav>
            <NetworkSelector />
          </div>
        </header>
        <main>{children}</main>
        <footer className="mt-12 border-t border-[var(--color-border)] pt-6 text-sm text-[var(--color-text-secondary)]">
          <LastUpdated />
        </footer>
      </div>
    </NetworkProvider>
  );
}
