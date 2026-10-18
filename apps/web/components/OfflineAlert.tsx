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
        "border-b border-[var(--color-warning)] bg-[var(--color-warning-bg)] px-4 py-2 text-sm",
        "backdrop-blur-sm",
        className,
      ].join(" ")}
    >
      <div className="flex items-center gap-2">
        {/* Pulse dot */}
        <span className="relative flex h-2.5 w-2.5 shrink-0">
          <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-[var(--color-warning)] opacity-75" />
          <span className="relative inline-flex h-2.5 w-2.5 rounded-full bg-[var(--color-warning)]" />
        </span>
        <span className="font-medium text-[var(--color-warning)]">You&apos;re offline</span>
        {pendingCount > 0 && (
          <span className="text-[var(--color-warning)]">
            &mdash; <strong className="font-semibold">{pendingCount}</strong>{" "}
            {pendingCount === 1 ? "alert" : "alerts"} queued
          </span>
        )}
      </div>
      {pendingCount > 0 && (
        <Link
          href="/offline-alerts"
          className="shrink-0 rounded bg-[var(--color-warning)] px-3 py-1 text-xs font-semibold text-[var(--color-bg-page)] transition-colors hover:opacity-90"
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
  Critical: "border-l-4 border-[var(--color-danger)] bg-[var(--color-danger-bg)]",
  Warning: "border-l-4 border-[var(--color-warning)] bg-[var(--color-warning-bg)]",
  Info: "border-l-4 border-[var(--color-info)] bg-[var(--color-info-bg)]",
};

const SEVERITY_BADGE: Record<QueuedAlert["severity"], string> = {
  Critical: "bg-[var(--color-danger-bg)] text-[var(--color-danger)]",
  Warning: "bg-[var(--color-warning-bg)] text-[var(--color-warning)]",
  Info: "bg-[var(--color-info-bg)] text-[var(--color-info)]",
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
                  <span
                    className="truncate font-mono text-xs text-[var(--color-text-secondary)]"
                    title={alert.contractId}
                  >
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
