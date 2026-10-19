/**
 * worker/index.ts — custom Service Worker logic for Sorolens PWA (issue #275).
 *
 * This file is compiled by @ducanh2912/next-pwa and injected into sw.js.
 * Handles:
 *  - Push notifications
 *  - Notification click routing
 *  - Offline alert background sync
 */

export {};

declare const self: any;

interface PushPayload {
  title: string;
  body: string;
  timestamp: string;
  contractId?: string;
  severity?: "Info" | "Warning" | "Critical";
  tag?: string;
  url?: string;
}

const SEVERITY_ICONS: Record<string, string> = {
  Critical: "/icons/icon-192.png",
  Warning: "/icons/icon-192.png",
  Info: "/icons/icon-192.png",
};

// ---- Push notification handler -----------------------------------------------

self.addEventListener("push", (event: any) => {
  let payload: PushPayload;
  try {
    payload = event.data?.json() as PushPayload;
  } catch {
    payload = {
      title: "Sorolens Alert",
      body: event.data?.text() ?? "New alert received.",
      timestamp: new Date().toISOString(),
    };
  }

  const notifOptions = {
    body: payload.body,
    icon: SEVERITY_ICONS[payload.severity ?? ""] ?? "/icons/icon-192.png",
    badge: "/icons/icon-192.png",
    tag: payload.tag ?? `sorolens-alert-${Date.now()}`,
    timestamp: new Date(payload.timestamp).getTime(),
    data: { url: payload.url ?? "/watchdog", contractId: payload.contractId },
    vibrate: payload.severity === "Critical" ? [200, 100, 200] : [100],
    requireInteraction: payload.severity === "Critical",
  };

  event.waitUntil(
    self.registration.showNotification(payload.title, notifOptions)
  );
});

// ---- Notification click -------------------------------------------------------

self.addEventListener("notificationclick", (event: any) => {
  event.notification.close();
  const url: string = event.notification.data?.url ?? "/watchdog";

  event.waitUntil(
    (async () => {
      const windowClients = await self.clients.matchAll({
        type: "window",
        includeUncontrolled: true,
      });
      for (const client of windowClients) {
        if ("focus" in client && "navigate" in client) {
          await client.navigate(url);
          await client.focus();
          return;
        }
      }
      await self.clients.openWindow(url);
    })()
  );
});

// ---- Offline queue sync -------------------------------------------------------
// The background sync tag is registered by useOfflineAlertQueue when the app
// detects it just came back online.

self.addEventListener("sync", (event: any) => {
  if (event.tag === "sorolens-alert-sync") {
    event.waitUntil(
      self.clients.matchAll({ type: "window" }).then((clients: any[]) => {
        clients.forEach((c) => c.postMessage({ type: "ALERT_SYNC" }));
      })
    );
  }
});
