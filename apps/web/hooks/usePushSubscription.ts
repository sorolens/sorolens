/**
 * usePushSubscription — manages Web Push subscription lifecycle (issue #275).
 *
 * Registers the service worker, subscribes to push using the VAPID public key,
 * posts the subscription to our /api/push/subscribe endpoint, and exposes a
 * `supported` flag so callers can gracefully degrade.
 */
"use client";

import { useCallback, useEffect, useState } from "react";

export type PushState =
  | "unsupported" // push API not in this browser
  | "denied" // permission explicitly denied
  | "idle" // not subscribed, no outstanding operation
  | "subscribing" // in-flight permission + subscription request
  | "subscribed" // actively subscribed
  | "error"; // last operation failed

export function usePushSubscription() {
  const [state, setState] = useState<PushState>("idle");
  const [error, setError] = useState<string | null>(null);

  // Detect support once on mount
  useEffect(() => {
    if (
      typeof window === "undefined" ||
      !("serviceWorker" in navigator) ||
      !("PushManager" in window)
    ) {
      setState("unsupported");
      return;
    }
    if (Notification.permission === "denied") {
      setState("denied");
    }
  }, []);

  /** Subscribe the current device to Web Push. */
  const subscribe = useCallback(async () => {
    setState("subscribing");
    setError(null);

    try {
      // Fetch the server's VAPID public key
      const keyRes = await fetch("/api/push/vapid-public-key");
      if (!keyRes.ok) throw new Error("Failed to fetch VAPID public key");
      const { publicKey } = (await keyRes.json()) as { publicKey: string };

      // Register (or reuse) the service worker
      const registration = await navigator.serviceWorker.ready;

      const permission = await Notification.requestPermission();
      if (permission !== "granted") {
        setState("denied");
        return;
      }

      // Subscribe via PushManager
      const subscription = await registration.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: urlBase64ToUint8Array(
          publicKey
        ) as unknown as BufferSource,
      });

      // Send subscription to our API
      const saveRes = await fetch("/api/push/subscribe", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(subscription.toJSON()),
      });
      if (!saveRes.ok) throw new Error("Failed to save subscription");

      setState("subscribed");
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setError(msg);
      setState("error");
    }
  }, []);

  /** Unsubscribe the current device. */
  const unsubscribe = useCallback(async () => {
    try {
      const registration = await navigator.serviceWorker.ready;
      const subscription = await registration.pushManager.getSubscription();
      if (subscription) {
        await subscription.unsubscribe();
        await fetch("/api/push/subscribe", {
          method: "DELETE",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ endpoint: subscription.endpoint }),
        });
      }
      setState("idle");
    } catch {
      // best-effort; don't surface unsubscribe errors to the user
    }
  }, []);

  return {
    state,
    error,
    isSupported: state !== "unsupported",
    isSubscribed: state === "subscribed",
    subscribe,
    unsubscribe,
  };
}

/** Convert a base64url string to a Uint8Array for PushManager.subscribe(). */
function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = "=".repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, "+").replace(/_/g, "/");
  const raw = atob(base64);
  return Uint8Array.from([...raw].map((c) => c.charCodeAt(0)));
}
