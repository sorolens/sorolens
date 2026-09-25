import Link from "next/link";

export default function NotFound() {
  return (
    <section className="flex min-h-[60vh] items-center justify-center py-16">
      <div className="w-full max-w-lg text-center">
        <p className="text-sm font-semibold uppercase tracking-[0.2em] text-[var(--color-accent)]">
          Contract not indexed
        </p>
        <h1 className="mt-4 text-5xl font-bold tracking-tight">404</h1>
        <h2 className="mt-3 text-2xl font-semibold">Nothing to observe here</h2>
        <p className="mt-4 text-[var(--color-text-secondary)]">
          We could not find that dashboard view. Return to your tracked
          contracts or head back to the Sorolens home page.
        </p>
        <div className="mt-8 flex flex-wrap justify-center gap-3">
          <Link
            href="/contracts"
            className="rounded-lg bg-[var(--color-accent)] px-5 py-2.5 text-sm font-semibold text-[var(--color-bg-page)] transition-opacity hover:opacity-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent)]"
          >
            View contracts
          </Link>
          <Link
            href="/"
            className="rounded-lg border border-[var(--color-border)] px-5 py-2.5 text-sm font-semibold transition-colors hover:border-[var(--color-accent)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent)]"
          >
            Back to home
          </Link>
        </div>
      </div>
    </section>
  );
}
