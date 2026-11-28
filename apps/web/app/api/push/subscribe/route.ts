import { createHash, timingSafeEqual } from "node:crypto";
import { NextResponse } from "next/server";
import { API_URL } from "@/lib/config";
import {
  sendPushNotification,
  type PushPayload,
  type PushSubscriptionJSON,
} from "@/lib/vapid";

export const runtime = "nodejs";

/**
 * Web Push routes.
 *
 * Subscriptions live in the Go API's `push_subscriptions` table (issue #592),
 * so a cold start no longer forgets every device. This route validates the
 * browser payload, then proxies registration and removal to the API using the
 * caller's workspace session. Dispatch (PUT) reads the persisted subscriptions
 * through the internal dispatch endpoint, guarded by PUSH_INTERNAL_SECRET.
 */

const maxBodyBytes = 8192;
const subscribeRate = rateLimit(60);
const sendRate = rateLimit(6);
let sending = false;

type DispatchSubscription = PushSubscriptionJSON & {
  filters?: {
    severity?: string;
    contract_id?: string;
    network?: string;
  };
};

function rateLimit(maxRequests: number) {
  let remaining = maxRequests;
  let resetAt = 0;
  return () => {
    const now = Date.now();
    if (now >= resetAt) {
      remaining = maxRequests;
      resetAt = now + 60_000;
    }
    if (remaining === 0) return Math.ceil((resetAt - now) / 1000);
    remaining--;
    return 0;
  };
}

function limited(retryAfter: number) {
  return NextResponse.json(
    { error: "Too many requests" },
    { status: 429, headers: { "Retry-After": String(retryAfter) } }
  );
}

function isObject(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

async function readPayload(request: Request) {
  if (Number(request.headers.get("content-length")) > maxBodyBytes)
    throw new Error("Payload too large");
  const reader = request.body?.getReader();
  if (!reader) throw new Error("Missing body");

  const decoder = new TextDecoder("utf-8", { fatal: true });
  let bytes = 0;
  let raw = "";
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      bytes += value.byteLength;
      if (bytes > maxBodyBytes) throw new Error("Payload too large");
      raw += decoder.decode(value, { stream: true });
    }
    const value: unknown = JSON.parse(raw + decoder.decode());
    if (!isObject(value)) throw new Error("Expected a JSON object");
    return value;
  } catch (err) {
    void reader.cancel().catch(() => {});
    throw err;
  } finally {
    reader.releaseLock();
  }
}

function validEndpoint(endpoint: unknown): endpoint is string {
  if (typeof endpoint !== "string" || endpoint.length > 2048) return false;
  try {
    const url = new URL(endpoint);
    return (
      url.protocol === "https:" &&
      !url.username &&
      !url.password &&
      !url.port &&
      !url.hash &&
      (url.hostname === "fcm.googleapis.com" ||
        url.hostname === "android.googleapis.com" ||
        url.hostname === "push.services.mozilla.com" ||
        url.hostname === "updates.push.services.mozilla.com" ||
        url.hostname.endsWith(".push.apple.com") ||
        url.hostname.endsWith(".notify.windows.com"))
    );
  } catch {
    return false;
  }
}

function validKey(value: unknown, bytes: number): value is string {
  if (typeof value !== "string" || !/^[A-Za-z0-9_-]+$/.test(value))
    return false;
  const decoded = Buffer.from(value, "base64url");
  return (
    decoded.length === bytes &&
    decoded.toString("base64url") === value &&
    (bytes !== 65 || decoded[0] === 4)
  );
}

function authorization(request: Request): string {
  return request.headers.get("authorization") ?? "";
}

/** Register a browser subscription. */
export async function POST(request: Request) {
  const retryAfter = subscribeRate();
  if (retryAfter) return limited(retryAfter);
  const auth = authorization(request);
  if (!auth) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }
  try {
    const { endpoint, keys, filters } = await readPayload(request);
    if (
      !validEndpoint(endpoint) ||
      !isObject(keys) ||
      !validKey(keys.auth, 16) ||
      !validKey(keys.p256dh, 65)
    ) {
      throw new Error("Invalid subscription");
    }

    const res = await fetch(`${API_URL}/api/v1/push/subscriptions`, {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: auth },
      body: JSON.stringify({
        endpoint,
        keys: { auth: keys.auth, p256dh: keys.p256dh },
        filters: isObject(filters) ? filters : undefined,
      }),
    });
    if (!res.ok) {
      return NextResponse.json(
        { error: "Could not save subscription" },
        { status: res.status === 401 ? 401 : 502 }
      );
    }
    return NextResponse.json({ ok: true }, { status: 201 });
  } catch {
    return NextResponse.json(
      { error: "Invalid subscription payload" },
      { status: 400 }
    );
  }
}

/** Remove a browser subscription. */
export async function DELETE(request: Request) {
  const retryAfter = subscribeRate();
  if (retryAfter) return limited(retryAfter);
  const auth = authorization(request);
  if (!auth) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }
  try {
    const { endpoint } = await readPayload(request);
    if (!validEndpoint(endpoint)) throw new Error("Invalid endpoint");

    const res = await fetch(`${API_URL}/api/v1/push/subscriptions`, {
      method: "DELETE",
      headers: { "Content-Type": "application/json", Authorization: auth },
      body: JSON.stringify({ endpoint }),
    });
    if (!res.ok) {
      return NextResponse.json(
        { error: "Could not remove subscription" },
        { status: res.status === 401 ? 401 : 502 }
      );
    }
    return NextResponse.json({ ok: true });
  } catch {
    return NextResponse.json({ error: "Invalid endpoint" }, { status: 400 });
  }
}

function optionalText(value: unknown, maxBytes: number): string | undefined {
  if (value === undefined || value === null) return undefined;
  if (typeof value !== "string" || Buffer.byteLength(value) > maxBytes)
    throw new Error("Invalid notification field");
  return value;
}

async function readNotification(request: Request): Promise<PushPayload> {
  const data = await readPayload(request);
  const severity = data.severity ?? undefined;
  if (
    severity !== undefined &&
    severity !== "Info" &&
    severity !== "Warning" &&
    severity !== "Critical"
  ) {
    throw new Error("Invalid severity");
  }
  const tag = optionalText(data.tag, 32);
  if (tag !== undefined && !/^[A-Za-z0-9_-]{1,32}$/.test(tag))
    throw new Error("Invalid notification tag");
  const timestamp =
    optionalText(data.timestamp, 64) ?? new Date().toISOString();
  if (!Number.isFinite(Date.parse(timestamp)))
    throw new Error("Invalid timestamp");

  const payload: PushPayload = {
    title: optionalText(data.title, 200) ?? "Sorolens Alert",
    body: optionalText(data.body, 2000) ?? "",
    timestamp,
    contractId: optionalText(data.contractId, 128),
    severity,
    url: optionalText(data.url, 1024),
    tag,
  };
  if (Buffer.byteLength(JSON.stringify(payload)) > 3800)
    throw new Error("Notification too large");
  return payload;
}

function hashSecret(value: string) {
  return createHash("sha256").update(value).digest();
}

function authorized(request: Request) {
  const secret = request.headers.get("x-push-secret");
  const expected = process.env.PUSH_INTERNAL_SECRET;
  return Boolean(
    expected &&
    secret &&
    timingSafeEqual(hashSecret(secret), hashSecret(expected))
  );
}

async function loadDispatchSubscriptions(): Promise<DispatchSubscription[]> {
  const res = await fetch(`${API_URL}/api/v1/push/subscriptions/dispatch`, {
    headers: { "X-Push-Secret": process.env.PUSH_INTERNAL_SECRET ?? "" },
    cache: "no-store",
  });
  if (!res.ok) throw new Error("Subscription store unavailable");
  const body = (await res.json()) as { subscriptions?: DispatchSubscription[] };
  return body.subscriptions ?? [];
}

async function pruneSubscription(endpoint: string) {
  try {
    await fetch(`${API_URL}/api/v1/push/subscriptions/dispatch`, {
      method: "DELETE",
      headers: {
        "Content-Type": "application/json",
        "X-Push-Secret": process.env.PUSH_INTERNAL_SECRET ?? "",
      },
      body: JSON.stringify({ endpoint }),
    });
  } catch {
    // Best effort: the next dispatch will retry the gone endpoint.
  }
}

function matchesFilters(
  filters: DispatchSubscription["filters"],
  notification: PushPayload
) {
  if (!filters) return true;
  if (
    filters.severity &&
    notification.severity &&
    filters.severity !== notification.severity
  ) {
    return false;
  }
  if (
    filters.contract_id &&
    notification.contractId &&
    filters.contract_id !== notification.contractId
  ) {
    return false;
  }
  return true;
}

/** Fan out a notification for the internal alerting pipeline. */
export async function PUT(request: Request) {
  if (!authorized(request)) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  if (sending) return limited(1);
  const retryAfter = sendRate();
  if (retryAfter) return limited(retryAfter);
  sending = true;
  try {
    let notification: PushPayload;
    try {
      notification = await readNotification(request);
    } catch {
      return NextResponse.json({ error: "Invalid payload" }, { status: 400 });
    }

    let recipients: DispatchSubscription[];
    try {
      recipients = await loadDispatchSubscriptions();
    } catch {
      return NextResponse.json(
        { error: "Subscription store unavailable" },
        { status: 503 }
      );
    }

    let sent = 0;
    let failed = 0;
    const targets = recipients.filter((sub) =>
      matchesFilters(sub.filters, notification)
    );
    for (let i = 0; i < targets.length; i += 20) {
      await Promise.all(
        targets.slice(i, i + 20).map(async (sub) => {
          try {
            await sendPushNotification(sub, notification);
            sent++;
          } catch (err) {
            failed++;
            const status = (err as { statusCode?: number } | null)?.statusCode;
            if (status === 404 || status === 410) {
              await pruneSubscription(sub.endpoint);
            }
          }
        })
      );
    }
    return NextResponse.json({ sent, failed });
  } finally {
    sending = false;
  }
}
