import Link from "next/link";
import Image from "next/image";
import { LiveStats } from "@/components/LiveStats";
import { CopyButton } from "@/components/CopyButton";

const cliQuickstart = `go install github.com/sorolens/sorolens/cli@latest
sorolens track CONTRACT_ID --network testnet`;

const apiQuickstart =
  "curl --fail-with-body 'https://api.sorolens.xyz/api/v1/contracts?limit=10'";

export const revalidate = 60;

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
      <nav
        aria-label="Main navigation"
        className="border-b border-[var(--color-border)]"
      >
        <div className="mx-auto flex max-w-7xl items-center justify-between px-4 py-4 sm:px-6 lg:px-8">
          <Link
            href="/"
            className="flex items-center gap-2 text-xl font-bold tracking-tight"
          >
            <Image src="/logo.svg" alt="" width={28} height={28} aria-hidden />
            Sorolens
          </Link>
          <div className="flex flex-wrap items-center justify-end gap-3 text-sm sm:gap-4">
            <Link
              href="/contracts"
              className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
            >
              Dashboard
            </Link>
            <Link
              href="/playground"
              className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
            >
              Playground
            </Link>
            <Link
              href="/compare"
              className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
            >
              Compare
            </Link>
            <Link
              href="/docs"
              className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
            >
              Docs
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

      <main>
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

        {/* Product screenshots */}
        <section className="border-y border-[var(--color-border)] bg-[var(--color-bg-card)]/40">
          <div className="mx-auto max-w-7xl px-4 py-16 sm:px-6 lg:px-8">
            <div className="mx-auto mb-10 max-w-2xl text-center">
              <h2 className="text-3xl font-bold tracking-tight">
                See what your data is doing
              </h2>
              <p className="mt-3 text-[var(--color-text-secondary)]">
                Follow contract activity, resource usage, and watchdog health in
                one place.
              </p>
            </div>
            <div className="grid gap-6 lg:grid-cols-3">
              {[
                {
                  src: "/screenshots/contracts.png",
                  alt: "Sorolens contracts dashboard with tracked contracts and activity",
                  label: "Contract dashboard",
                },
                {
                  src: "/screenshots/contract-detail.png",
                  alt: "Sorolens contract detail dashboard with events and resource trends",
                  label: "Resource trends",
                },
                {
                  src: "/screenshots/watchdog.png",
                  alt: "Sorolens watchdog overview showing monitored contract health",
                  label: "Watchdog health",
                },
              ].map((shot) => (
                <figure
                  key={shot.src}
                  className="overflow-hidden rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-card)]"
                >
                  <Image
                    src={shot.src}
                    alt={shot.alt}
                    width={800}
                    height={1000}
                    sizes="(max-width: 1024px) 100vw, 33vw"
                    className="h-64 w-full object-cover object-top sm:h-80"
                  />
                  <figcaption className="px-4 py-3 text-sm font-medium">
                    {shot.label}
                  </figcaption>
                </figure>
              ))}
            </div>
          </div>
        </section>

        {/* Quickstart */}
        <section className="mx-auto max-w-7xl px-4 py-20 sm:px-6 lg:px-8">
          <div className="mx-auto mb-10 max-w-2xl text-center">
            <h2 className="text-3xl font-bold tracking-tight">
              Start with a contract
            </h2>
            <p className="mt-3 text-[var(--color-text-secondary)]">
              Install the CLI to track a Soroban contract, or query the API
              directly. Replace <code>CONTRACT_ID</code> with your contract
              address.
            </p>
          </div>
          <div className="grid gap-6 lg:grid-cols-2">
            {[
              { title: "CLI quickstart", command: cliQuickstart, label: "CLI" },
              { title: "Query the API", command: apiQuickstart, label: "API" },
            ].map((example) => (
              <section
                key={example.label}
                aria-label={example.title}
                className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-card)] p-5"
              >
                <div className="mb-3 flex items-center justify-between gap-3">
                  <h3 className="font-semibold">{example.title}</h3>
                  <CopyButton
                    text={example.command}
                    label={`Copy ${example.title}`}
                  />
                </div>
                <pre className="overflow-x-auto rounded-lg bg-black/30 p-4 text-xs leading-relaxed sm:text-sm">
                  <code>{example.command}</code>
                </pre>
              </section>
            ))}
          </div>
          <p className="mt-5 text-center text-sm text-[var(--color-text-secondary)]">
            New to Sorolens?{" "}
            <Link
              href="/docs"
              className="font-medium text-[var(--color-accent)] underline underline-offset-4"
            >
              Read the getting started guide
            </Link>
            .
          </p>
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

        {/* Explore the product */}
        <section className="border-t border-[var(--color-border)]">
          <div className="mx-auto max-w-7xl px-4 py-16 sm:px-6 lg:px-8">
            <h2 className="mb-8 text-center text-3xl font-bold tracking-tight">
              Explore Sorolens
            </h2>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              {[
                { href: "/playground", label: "API Playground" },
                { href: "/compare", label: "Compare contracts" },
                { href: "/watchdog/reports", label: "Health reports" },
                { href: "/docs", label: "Documentation" },
              ].map((destination) => (
                <Link
                  key={destination.href}
                  href={destination.href}
                  className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-5 text-center font-semibold transition-colors hover:border-[var(--color-accent)] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent)]"
                >
                  {destination.label}
                </Link>
              ))}
            </div>
          </div>
        </section>
      </main>

      {/* Footer */}
      <footer className="border-t border-[var(--color-border)]">
        <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-3 px-4 py-8 text-sm text-[var(--color-text-secondary)] sm:px-6 lg:px-8">
          <span>MIT licensed. Built for the Stellar developer community.</span>
          <nav aria-label="Footer navigation" className="flex flex-wrap gap-4">
            <Link href="/docs">Docs</Link>
            <Link href="/playground">Playground</Link>
            <Link href="/compare">Compare</Link>
            <Link href="/watchdog/reports">Reports</Link>
            <a
              href="https://github.com/sorolens/sorolens"
              rel="noopener noreferrer"
            >
              Monorepo
            </a>
            <a
              href="https://github.com/sorolens/sorolens/tree/main/cli"
              rel="noopener noreferrer"
            >
              CLI
            </a>
            <a
              href="https://github.com/sorolens/sorolens/tree/main/packages"
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
          </nav>
        </div>
      </footer>
    </div>
  );
}
