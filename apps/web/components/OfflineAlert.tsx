"use client";

/**
 * OfflineBanner — shown when navigator.onLine is false.
 *
 * Rendered at the top of every app page.  When offline it shows how many
 * alerts are queued in IndexedDB and links to the offline-alerts panel.
 */
import Link from "next/link";
import type { QueuedAlert } from "@/lib/alertQueue";

interface Props {
  isOffline: boolean;
  pendingCount: number;
  /** Optional extra class names. */
  className?: string;
}

export function OfflineBanner({
  isOffline,
  pendingCount,
  className = "",
}: Props) {
  if (!isOffline) return null;

  return (
    <div
      id="offline-banner"
      role="alert"
      aria-live="assertive"
      aria-atomic="true"
      className={[
        "sticky top-0 z-50 flex items-center justify-between gap-3",
        "border-b border-amber-700 bg-amber-950/90 px-4 py-2 text-sm",
        "backdrop-blur-sm",
        className,
      ].join(" ")}
    >
      <div className="flex items-center gap-2">
        {/* Pulse dot */}
        <span className="relative flex h-2.5 w-2.5 shrink-0">
          <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-amber-400 opacity-75" />
          <span className="relative inline-flex h-2.5 w-2.5 rounded-full bg-amber-500" />
        </span>
        <span className="font-medium text-amber-200">You&apos;re offline</span>
        {pendingCount > 0 && (
          <span className="text-amber-300">
            &mdash; <strong className="font-semibold">{pendingCount}</strong>{" "}
            {pendingCount === 1 ? "alert" : "alerts"} queued
          </span>
        )}
      </div>
      {pendingCount > 0 && (
        <Link
          href="/offline-alerts"
          className="shrink-0 rounded bg-amber-700 px-3 py-1 text-xs font-semibold text-amber-100 transition-colors hover:bg-amber-600"
        >
          View queue
        </Link>
      )}
    </div>
  );
}

/**
 * OfflineAlertPanel — renders a list of queued offline alerts.
 *
 * Used on the /offline-alerts page.  Receives alerts from the IDB queue via
 * useOfflineAlertQueue and exposes dismiss / dismiss-all controls.
 */
interface PanelProps {
  alerts: QueuedAlert[];
  onDismiss: (id: string) => void;
  onDismissAll: () => void;
}

const SEVERITY_STYLES: Record<QueuedAlert["severity"], string> = {
  Critical: "border-l-4 border-red-500 bg-red-950/40",
  Warning: "border-l-4 border-amber-500 bg-amber-950/30",
  Info: "border-l-4 border-sky-500 bg-sky-950/30",
};

const SEVERITY_BADGE: Record<QueuedAlert["severity"], string> = {
  Critical: "bg-red-900 text-red-200",
  Warning: "bg-amber-900 text-amber-200",
  Info: "bg-sky-900 text-sky-200",
};

export function OfflineAlertPanel({
  alerts,
  onDismiss,
  onDismissAll,
}: PanelProps) {
  if (alerts.length === 0) {
    return (
      <div
        id="offline-alert-panel-empty"
        className="flex flex-col items-center justify-center gap-3 py-16 text-[var(--color-text-secondary)]"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          className="h-12 w-12 opacity-30"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          aria-hidden
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={1.5}
            d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
          />
        </svg>
        <p className="text-sm">No queued alerts — all caught up.</p>
      </div>
    );
  }

  return (
    <section id="offline-alert-panel" aria-label="Offline alert queue">
      <div className="mb-4 flex items-center justify-between gap-4">
        <p className="text-sm text-[var(--color-text-secondary)]">
          {alerts.length} {alerts.length === 1 ? "alert" : "alerts"} received
          while offline
        </p>
        <button
          id="dismiss-all-btn"
          onClick={onDismissAll}
          className="rounded border border-[var(--color-border)] px-3 py-1 text-xs font-medium text-[var(--color-text-secondary)] transition-colors hover:border-[var(--color-accent)] hover:text-[var(--color-accent)]"
        >
          Dismiss all
        </button>
      </div>
      <ul className="space-y-3" aria-label="Queued alerts list">
        {alerts.map((alert) => (
          <li
            key={alert.id}
            id={`offline-alert-${alert.id}`}
            className={[
              "group relative rounded-lg p-4 transition-opacity",
              SEVERITY_STYLES[alert.severity],
            ].join(" ")}
          >
            <div className="flex items-start justify-between gap-4">
              <div className="min-w-0 flex-1">
                <div className="mb-1 flex flex-wrap items-center gap-2">
                  <span
                    className={[
                      "rounded-full px-2 py-0.5 text-xs font-semibold uppercase",
                      SEVERITY_BADGE[alert.severity],
                    ].join(" ")}
                  >
                    {alert.severity}
                  </span>
                  <span className="truncate font-mono text-xs text-[var(--color-text-secondary)]">
                    {alert.contractId}
                  </span>
                </div>
                <p className="text-sm leading-relaxed text-[var(--color-text-primary)]">
                  {alert.message}
                </p>
                <p className="mt-1.5 text-xs text-[var(--color-text-secondary)]">
                  Queued{" "}
                  {new Date(alert.enqueuedAt).toLocaleString(undefined, {
                    dateStyle: "short",
                    timeStyle: "short",
                  })}
                </p>
              </div>
              <button
                id={`dismiss-btn-${alert.id}`}
                onClick={() => onDismiss(alert.id)}
                aria-label={`Dismiss alert ${alert.id}`}
                className="mt-0.5 shrink-0 rounded p-1 text-[var(--color-text-secondary)] opacity-0 transition-all hover:text-[var(--color-text-primary)] group-hover:opacity-100"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  className="h-4 w-4"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  aria-hidden
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M6 18L18 6M6 6l12 12"
                  />
                </svg>
              </button>
            </div>
          </li>
        ))}
      </ul>
    </section>
  );
}
