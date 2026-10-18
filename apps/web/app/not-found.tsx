import Link from "next/link";

export default function NotFound() {
  return (
    <main className="flex min-h-screen items-center justify-center px-6 py-16">
      <section className="w-full max-w-lg text-center">
        <p className="text-sm font-semibold uppercase tracking-[0.2em] text-[var(--color-accent)]">
          Signal not found
        </p>
        <h1 className="mt-4 text-5xl font-bold tracking-tight">404</h1>
        <h2 className="mt-3 text-2xl font-semibold">This page is off-chain</h2>
        <p className="mt-4 text-[var(--color-text-secondary)]">
          The page may have moved, or the address may be incorrect. Return to
          Sorolens to keep exploring the network.
        </p>
        <Link
          href="/"
          className="mt-8 inline-flex rounded-lg bg-[var(--color-accent)] px-5 py-2.5 text-sm font-semibold text-[var(--color-bg-page)] transition-opacity hover:opacity-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent)]"
        >
          Back to home
        </Link>
      </section>
    </main>
  );
}
