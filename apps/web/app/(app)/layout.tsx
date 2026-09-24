"use client";

import Link from "next/link";
import { NetworkProvider } from "@/lib/network";
import { NetworkSelector } from "@/components/NetworkSelector";
import { CommandPalette, useCommandPalette } from "@/components/CommandPalette";

function AppShell({ children }: { children: React.ReactNode }) {
  const { open, setOpen } = useCommandPalette();

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
            {/* Cmd+K trigger button */}
            <button
              type="button"
              id="cmd-k-trigger"
              onClick={() => setOpen(true)}
              aria-label="Open command palette (Cmd+K)"
              className="hidden items-center gap-1.5 rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] px-3 py-1.5 text-xs text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)] sm:flex"
            >
              <svg
                className="h-3.5 w-3.5"
                viewBox="0 0 20 20"
                fill="currentColor"
                aria-hidden="true"
              >
                <path
                  fillRule="evenodd"
                  d="M9 3.5a5.5 5.5 0 100 11 5.5 5.5 0 000-11zM2 9a7 7 0 1112.452 4.391l3.328 3.329a.75.75 0 11-1.06 1.06l-3.329-3.328A7 7 0 012 9z"
                  clipRule="evenodd"
                />
              </svg>
              <span>Search…</span>
              <kbd className="rounded border border-[var(--color-border)] px-1 py-px font-mono text-[10px]">
                ⌘K
              </kbd>
            </button>
            <NetworkSelector />
          </div>
        </header>
        <main>{children}</main>
      </div>

      {open && (
        <CommandPalette onClose={() => setOpen(false)} />
      )}
    </NetworkProvider>
  );
}

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return <AppShell>{children}</AppShell>;
}
