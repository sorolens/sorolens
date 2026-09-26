import Link from "next/link";
import { LiveStats } from "@/components/LiveStats";

const features = [
  {
    title: "Contract Event Indexing",
    body: "Every event your contract emits (decoded topics, decoded values, ledger and transaction context) indexed into Postgres and queryable via REST or the dashboard.",
  },
  {
    title: "Storage Tracking",
    body: "Snapshot of every temporary, persistent, and instance storage entry, with TTL health so you see which keys are about to expire before your users do.",
  },
  {
    title: "Invocation Tracing",
    body: "Per-transaction CPU instructions, memory, ledger I/O bytes, and fee charged: the numbers you need to catch a regression before mainnet.",
  },
  {
    title: "Watchdog Monitoring",
    body: "On-chain health checks and alerts, published by our sorolens-watchdog Soroban contract. Register your contract, push status updates, and see the timeline here.",
    badge: "New",
  },
];

const steps = [
  {
    n: "1",
    title: "Track a contract",
    body: "Point the CLI or the API at any Soroban contract on testnet, mainnet, or futurenet. The indexer starts filling in its history on the next tick.",
  },
  {
    n: "2",
    title: "Watch it live",
    body: "Events, invocations, and storage state land in the dashboard as the indexer pulls them from RPC, with a REST API in front for CI, alerts, or your own tooling.",
  },
  {
    n: "3",
    title: "Add a watchdog",
    body: "For proactive monitoring, register your contract with the on-chain watchdog and push status updates. Any downtime, degraded response, or alert shows up here immediately.",
  },
];

export default function HomePage() {
  return (
    <div className="min-h-screen">
      {/* Top nav */}
      <nav className="border-b border-[var(--color-border)]">
        <div className="mx-auto flex max-w-7xl items-center justify-between px-4 py-4 sm:px-6 lg:px-8">
          <Link
            href="/"
            className="flex items-center gap-2 text-xl font-bold tracking-tight"
          >
            <img src="/logo.svg" alt="" className="h-7 w-7" aria-hidden />
            Sorolens
          </Link>
          <div className="flex items-center gap-4 text-sm">
            <Link
              href="/contracts"
              className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
            >
              Dashboard
            </Link>
            <a
              href="https://github.com/sorolens/sorolens"
              className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
              rel="noopener noreferrer"
            >
              GitHub
            </a>
          </div>
        </div>
      </nav>

      {/* Hero */}
      <section className="mx-auto max-w-7xl px-4 py-24 sm:px-6 lg:px-8">
        <div className="mx-auto max-w-3xl text-center">
          <span className="mb-4 inline-block rounded-full border border-[var(--color-border)] px-3 py-1 text-xs uppercase tracking-wider text-[var(--color-text-secondary)]">
            Soroban observability
          </span>
          <h1 className="mb-6 text-5xl font-bold leading-tight tracking-tight sm:text-6xl">
            Real-time monitoring and on-chain health checks for Soroban
            contracts.
          </h1>
          <p className="mb-8 text-lg text-[var(--color-text-secondary)]">
            The only Stellar observability tool with a deployed Soroban{" "}
            <em>watchdog</em> contract for proactive contract monitoring.
            Events, invocations, storage TTLs, and health status: indexed,
            queryable, and alertable.
          </p>
          <div className="flex flex-wrap items-center justify-center gap-3">
            <Link
              href="/contracts"
              className="rounded-lg bg-[var(--color-accent)] px-6 py-3 text-sm font-semibold text-[var(--color-bg-page)] transition-opacity hover:opacity-90"
            >
              View Dashboard
            </Link>
            <a
              href="https://github.com/sorolens/sorolens"
              className="rounded-lg border border-[var(--color-border)] px-6 py-3 text-sm font-semibold text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-accent)]"
              rel="noopener noreferrer"
            >
              View on GitHub
            </a>
          </div>
        </div>

        {/* Live stats */}
        <LiveStats className="mx-auto mt-16 max-w-4xl" />
      </section>

      {/* Features */}
      <section className="border-t border-[var(--color-border)]">
        <div className="mx-auto max-w-7xl px-4 py-20 sm:px-6 lg:px-8">
          <div className="grid gap-6 sm:grid-cols-2">
            {features.map((f) => (
              <div
                key={f.title}
                className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-6"
              >
                <div className="mb-2 flex items-center gap-2">
                  <h3 className="text-lg font-semibold">{f.title}</h3>
                  {f.badge && (
                    <span className="rounded-full bg-[var(--color-accent)] px-2 py-0.5 text-xs font-semibold text-[var(--color-bg-page)]">
                      {f.badge}
                    </span>
                  )}
                </div>
                <p className="text-sm leading-relaxed text-[var(--color-text-secondary)]">
                  {f.body}
                </p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* How it works */}
      <section className="border-t border-[var(--color-border)]">
        <div className="mx-auto max-w-7xl px-4 py-20 sm:px-6 lg:px-8">
          <h2 className="mb-10 text-center text-3xl font-bold tracking-tight">
            How it works
          </h2>
          <div className="grid gap-6 sm:grid-cols-3">
            {steps.map((s) => (
              <div key={s.n} className="text-center">
                <div className="mx-auto mb-4 flex h-10 w-10 items-center justify-center rounded-full border border-[var(--color-accent)] text-sm font-semibold text-[var(--color-accent)]">
                  {s.n}
                </div>
                <h3 className="mb-2 font-semibold">{s.title}</h3>
                <p className="text-sm leading-relaxed text-[var(--color-text-secondary)]">
                  {s.body}
                </p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-[var(--color-border)]">
        <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-3 px-4 py-8 text-sm text-[var(--color-text-secondary)] sm:px-6 lg:px-8">
          <span>MIT licensed. Built for the Stellar developer community.</span>
          <div className="flex flex-wrap gap-4">
            <a
              href="https://github.com/sorolens/sorolens"
              rel="noopener noreferrer"
            >
              Monorepo
            </a>
            <a
              href="https://github.com/sorolens/sorolens-cli"
              rel="noopener noreferrer"
            >
              CLI
            </a>
            <a
              href="https://github.com/sorolens/sorolens-sdk"
              rel="noopener noreferrer"
            >
              SDK
            </a>
            <a
              href="https://github.com/sorolens/sorolens/blob/main/CONTRIBUTING.md"
              rel="noopener noreferrer"
            >
              Contribute
            </a>
            <a href="https://discord.gg/D9jATUezYX" rel="noopener noreferrer">
              Discord
            </a>
            <a href="https://t.me/sorolens_community" rel="noopener noreferrer">
              Telegram
            </a>
          </div>
        </div>
      </footer>
    </div>
  );
}
