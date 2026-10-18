"use client";

import * as Sentry from "@sentry/nextjs";
import { useEffect } from "react";

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    Sentry.captureException(error, {
      tags: {
        routeGroup: "root",
      },
    });
  }, [error]);

  return (
    <main className="flex min-h-screen items-center justify-center px-6 py-16">
      <section
        className="w-full max-w-lg rounded-2xl border border-[var(--color-border)] bg-[var(--color-bg-card)] p-8 text-center shadow-2xl"
        role="alert"
      >
        <div
          className="mx-auto mb-5 flex h-12 w-12 items-center justify-center rounded-full border border-[var(--color-border)] bg-[var(--color-bg-page)] text-xl text-[var(--color-warning)]"
          aria-hidden
        >
          !
        </div>
        <h1 className="text-2xl font-bold tracking-tight">
          This page hit a snag
        </h1>
        <p className="mt-3 text-[var(--color-text-secondary)]">
          Sorolens could not load this page. Your data is safe—please try again.
        </p>
        <button
          type="button"
          onClick={reset}
          className="mt-7 rounded-lg bg-[var(--color-accent)] px-5 py-2.5 text-sm font-semibold text-[var(--color-bg-page)] transition-opacity hover:opacity-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent)]"
        >
          Try again
        </button>
      </section>
    </main>
  );
}
