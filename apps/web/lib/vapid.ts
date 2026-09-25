/**
 * VAPID Web Push helpers (issue #275).
 *
 * Server-side only. Never import this from client components.
 *
 * Environment variables required:
 *   VAPID_PUBLIC_KEY   – base64url-encoded 65-byte uncompressed EC P-256 key
 *   VAPID_PRIVATE_KEY  – base64url-encoded 32-byte EC P-256 private key
 *   VAPID_SUBJECT      – mailto: or https: contact URI for the VAPID JWT
 *
 * Generate a key-pair once with:
 *   node -e "const wp=require('web-push'); const k=wp.generateVAPIDKeys(); console.log(k);"
 */

import webPush from "web-push";

export interface PushSubscriptionJSON {
  endpoint: string;
  keys: {
    auth: string;
    p256dh: string;
  };
}

export interface PushPayload {
  title: string;
  body: string;
  /** ISO-8601 timestamp. */
  timestamp: string;
  contractId?: string;
  severity?: "Info" | "Warning" | "Critical";
  tag?: string;
  url?: string;
}

function getVapidDetails() {
  const publicKey = process.env.VAPID_PUBLIC_KEY;
  const privateKey = process.env.VAPID_PRIVATE_KEY;
  const subject = process.env.VAPID_SUBJECT || "mailto:admin@sorolens.dev";

  if (!publicKey || !privateKey) {
    throw new Error(
      "VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY must be set. " +
        "Generate them with: node -e \"const wp=require('web-push'); console.log(wp.generateVAPIDKeys())\""
    );
  }

  return { publicKey, privateKey, subject };
}

/**
 * Send a Web Push notification to a single subscription.
 * Throws on 4xx/5xx errors (caller should handle 410 Gone by deleting the subscription).
 */
export async function sendPushNotification(
  subscription: PushSubscriptionJSON,
  payload: PushPayload
): Promise<void> {
  const { publicKey, privateKey, subject } = getVapidDetails();

  webPush.setVapidDetails(subject, publicKey, privateKey);

  await webPush.sendNotification(
    subscription as Parameters<typeof webPush.sendNotification>[0],
    JSON.stringify(payload),
    {
      TTL: 86400, // 24 hours
      urgency: payload.severity === "Critical" ? "high" : "normal",
      topic: payload.tag,
    }
  );
}

/** Public VAPID key for the client-side `applicationServerKey`. */
export function getPublicVapidKey(): string {
  const { publicKey } = getVapidDetails();
  return publicKey;
}
