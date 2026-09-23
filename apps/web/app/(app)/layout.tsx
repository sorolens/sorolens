import Link from "next/link";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
      <header className="mb-8 flex flex-wrap items-center justify-between gap-4">
        <Link
          href="/"
          className="flex items-center gap-2 text-2xl font-bold tracking-tight"
        >
          <img src="/logo.svg" alt="" className="h-8 w-8" aria-hidden />
          Sorolens
        </Link>
        <nav className="flex gap-4 text-sm">
          <Link
            href="/contracts"
            className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
          >
            Contracts
          </Link>
          <Link
            href="/watchlist"
            className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
          >
            Watchlist
          </Link>
          <Link
            href="/watchdog"
            className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
          >
            Watchdog
          </Link>
          <Link
            href="/compare"
            className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
          >
            Compare
          </Link>
        </nav>
      </header>
      <main>{children}</main>
    </div>
  );
}
