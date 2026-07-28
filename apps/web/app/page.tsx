import Link from "next/link";

export default function HomePage() {
  return (
    <div className="flex flex-col items-center justify-center py-24">
      <h1 className="mb-4 text-4xl font-bold">Sorolens</h1>
      <p className="mb-8 text-lg text-[var(--color-text-secondary)]">
        Indexed observability for Soroban smart contracts
      </p>
      <Link
        href="/contracts"
        className="rounded-lg bg-[var(--color-accent)] px-6 py-3 text-sm font-semibold text-[var(--color-bg-page)] transition-opacity hover:opacity-90"
      >
        View Contracts
      </Link>
    </div>
  );
}
