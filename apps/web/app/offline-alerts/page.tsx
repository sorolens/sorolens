"use client";

/**
 * /offline-alerts — queued-alert view (issue #275 AC4)
 *
 * Displays all non-dismissed alerts that were captured in IndexedDB
 * while the device was offline.  The data comes from useOfflineAlertQueue
 * which reads from the same IDB store the service worker writes to via
 * push events.
 */
import Link from "next/link";
import { useOfflineAlertQueue } from "@/hooks/useOfflineAlertQueue";
import { OfflineAlertPanel } from "@/components/OfflineAlert";

export default function OfflineAlertsPage() {
  const { pending, dismiss, dismissAll } = useOfflineAlertQueue();

  return (
    <div className="mx-auto max-w-3xl px-4 py-8 sm:px-6 lg:px-8">
      <div className="mb-6 flex items-center gap-4">
        <Link
          href="/"
          aria-label="Back"
          className="flex h-8 w-8 items-center justify-center rounded-full border border-[var(--color-border)] text-[var(--color-text-secondary)] transition-colors hover:border-[var(--color-accent)] hover:text-[var(--color-accent)]"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            className="h-4 w-4"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth={2}
            aria-hidden
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M15 19l-7-7 7-7"
            />
          </svg>
        </Link>
        <div>
          <h1 className="text-xl font-bold tracking-tight">
            Offline Alert Queue
          </h1>
          <p className="mt-0.5 text-sm text-[var(--color-text-secondary)]">
            Alerts received while offline — backed by IndexedDB, survives page
            reloads.
          </p>
        </div>
      </div>

      <OfflineAlertPanel
        alerts={pending}
        onDismiss={dismiss}
        onDismissAll={dismissAll}
      />
    </div>
  );
}
