"use client";

/**
 * PushSubscribeButton — lets the user subscribe/unsubscribe from Web Push.
 *
 * Placed in the app header. Only renders when push is supported in this
 * browser; silently returns null otherwise.
 */
import { usePushSubscription } from "@/hooks/usePushSubscription";

export function PushSubscribeButton() {
  const { state, error, isSupported, subscribe, unsubscribe } =
    usePushSubscription();

  if (!isSupported || state === "unsupported") return null;

  const label: Record<string, string> = {
    idle: "Enable alerts",
    subscribing: "Enabling…",
    subscribed: "Alerts on",
    denied: "Blocked",
    error: "Retry alerts",
  };

  return (
    <div className="relative">
      <button
        id="push-subscribe-btn"
        onClick={state === "subscribed" ? unsubscribe : subscribe}
        disabled={state === "subscribing"}
        aria-label={
          state === "subscribed" ? "Disable push alerts" : "Enable push alerts"
        }
        title={error ?? undefined}
        className={[
          "flex items-center gap-1.5 rounded border px-3 py-1.5 text-xs font-medium transition-all",
          state === "subscribed"
            ? "border-[var(--color-accent)] text-[var(--color-accent)] hover:bg-[var(--color-accent)]/10"
            : "border-[var(--color-border)] text-[var(--color-text-secondary)] hover:border-[var(--color-accent)] hover:text-[var(--color-accent)]",
          state === "subscribing" && "opacity-60",
        ].join(" ")}
      >
        {/* Bell icon */}
        <svg
          xmlns="http://www.w3.org/2000/svg"
          className="h-3.5 w-3.5"
          viewBox="0 0 24 24"
          fill={state === "subscribed" ? "currentColor" : "none"}
          stroke="currentColor"
          strokeWidth={2}
          aria-hidden
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"
          />
        </svg>
        <span>{label[state] ?? "Alerts"}</span>
      </button>
    </div>
  );
}
