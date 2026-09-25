"use client";

import { usePushSubscription } from "@/hooks/usePushSubscription";

/**
 * PushNotificationToggle
 *
 * A compact control that lets users subscribe to or unsubscribe from Web Push
 * notifications for Sorolens alerts (issue #275).  Rendered in the app shell
 * header.
 *
 * Falls back gracefully when:
 *   – the browser doesn't support Push  (button hidden)
 *   – VAPID keys aren't configured      (error shown)
 *   – the user denies permission        (denied state shown)
 */
export function PushNotificationToggle() {
  const { state, error, isSupported, isSubscribed, subscribe, unsubscribe } =
    usePushSubscription();

  if (!isSupported || state === "unsupported") return null;

  return (
    <div
      id="push-notification-toggle"
      style={{ display: "flex", alignItems: "center", gap: "0.5rem" }}
    >
      {state === "denied" && (
        <span
          style={{ fontSize: "0.72rem", color: "#94a3b8" }}
          title="Push notifications blocked. Enable them in your browser settings."
        >
          🔕 Notifications blocked
        </span>
      )}

      {state === "error" && error && (
        <span style={{ fontSize: "0.72rem", color: "#f87171" }} title={error}>
          Push unavailable
        </span>
      )}

      {state !== "denied" && (
        <button
          id={isSubscribed ? "push-unsubscribe-btn" : "push-subscribe-btn"}
          onClick={isSubscribed ? unsubscribe : subscribe}
          disabled={state === "subscribing"}
          aria-label={
            isSubscribed
              ? "Unsubscribe from push notifications"
              : "Subscribe to push notifications"
          }
          style={{
            display: "flex",
            alignItems: "center",
            gap: "0.35rem",
            padding: "0.35rem 0.75rem",
            borderRadius: "0.5rem",
            border: "1px solid",
            borderColor: isSubscribed
              ? "rgba(124,58,237,0.5)"
              : "rgba(255,255,255,0.12)",
            background: isSubscribed ? "rgba(124,58,237,0.15)" : "transparent",
            color: isSubscribed ? "#a78bfa" : "#94a3b8",
            fontSize: "0.78rem",
            fontWeight: 500,
            cursor: state === "subscribing" ? "wait" : "pointer",
            transition: "all 0.15s",
            whiteSpace: "nowrap",
          }}
        >
          <span aria-hidden style={{ fontSize: "0.85rem" }}>
            {state === "subscribing" ? "⏳" : isSubscribed ? "🔔" : "🔕"}
          </span>
          {state === "subscribing"
            ? "Enabling…"
            : isSubscribed
              ? "Alerts on"
              : "Enable alerts"}
        </button>
      )}
    </div>
  );
}
