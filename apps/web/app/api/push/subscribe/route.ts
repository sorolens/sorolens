import { NextRequest, NextResponse } from "next/server";
import { timingSafeEqual } from "crypto";
import { sendPushNotification, type PushSubscriptionJSON } from "@/lib/vapid";

/**
 * In-memory subscription store.
 *
 * NOTE: This is a development-only stub.  In production, persist subscriptions
 * in a database (e.g., Postgres via the existing apps/api store) and associate
 * them with a user / contract filter.  The in-process store is lost on each
 * serverless cold start.
 *
 * To wire in a persistent store, replace the Map operations below with DB calls.
 */
const subscriptions = new Map<string, PushSubscriptionJSON>();

/**
 * Constant-time comparison of the provided push secret against the
 * configured PUSH_INTERNAL_SECRET.  Returns false when the secret is
 * unset (fail closed) or when the provided value is missing/different.
 */
function verifyPushSecret(provided: string | null): boolean {
  const expected = process.env.PUSH_INTERNAL_SECRET;
  if (!expected) return false;
  if (!provided) return false;
  const a = Buffer.from(provided, "utf8");
  const b = Buffer.from(expected, "utf8");
  if (a.length !== b.length) return false;
  return timingSafeEqual(a, b);
}

/**
 * POST /api/push/subscribe
 * Body: PushSubscription JSON (endpoint + keys)
 *
 * Saves the subscription so the server can send push notifications later.
 */
export async function POST(request: NextRequest) {
  try {
    const body = (await request.json()) as PushSubscriptionJSON;

    if (!body?.endpoint || !body?.keys?.auth || !body?.keys?.p56dh) {
      return NextResponse.json({ error: "Invalid subscription payload" }, { status: 400 });
    }

    subscriptions.set(body.endpoint, body);
    return NextResponse.json({ ok: true }, { status: 201 });
  } catch {
    return NextResponse.json({ error: "Bad request" }, { status: 400 });
  }
}

/**
 * DELETE /api/push/subscribe
 * Body: { endpoint: string }
 *
 * Removes a subscription.
 */
export async function DELETE(request: NextRequest) {
  try {
    const body = (await request.json()) as { endpoint: string };
    if (!body?.endpoint) {
      return NextResponse.json({ error: "endpoint required" }, { status: 400 });
    }
    subscriptions.delete(body.endpoint);
    return NextResponse.json({ ok: true });
  } catch {
    return NextResponse.json({ error: "Bad request" }, { status: 400 });
  }
}

/**
 * PUT /api/push/subscribe  (internal — called by the alerting pipeline, issue #127)
 *
 * When issue #127 is fully merged, the server-side alert delivery service
 * should call this endpoint (or use the vapid.ts helper directly) to fan out
 * push notifications to all subscribed clients.
 *
 * Accepts: { title, body, timestamp, contractId?, severity?, url? }
 * Returns: { sent: number, failed: number }
 *
 * This route is intentionally NOT public. It requires the PUSH_INTERNAL_SECRET
 * env var to be set and the matching value to be provided in the "x-push-secret"
 * header. If the secret is unset, the route fails closed with 503.
 */
export async function PUT(request: NextRequest) {
  const expected = process.env.PUSH_INTERNAL_SECRET;
  if (!expected) {
    return NextResponse.json({ error: "Push secret not configured" }, { status: 503 });
  }

  const provided = request.headers.get("x-push-secret");
  if (!verifyPushSecret(provided)) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  let payload: {
    title?: string;
    body?: string;
    timestamp?: string;
    contractId?: string;
    severity?: string;
    url?: string;
    tag?: string;
  };
  try {
    payload = await request.json();
  } catch {
    return NextResponse.json({ error: "Bad request" }, { status: 400 });
  }

  let sent = 0;
  let failed = 0;

  await Promise.all(
    Array.from(subscriptions.values()).map(async (sub) => {
      try {
        await sendPushNotification(sub, {
          title: payload.title ?? "Sorolens Alert",
          body: payload.body ?? "",
          timestamp: payload.timestamp ?? new Date().toISOString(),
          contractId: payload.contractId,
          severity: payload.severity,
          url: payload.url,
          tag: payload.tag,
        });
        sent++;
      } catch (err) {
        failed++;
        // Remove gone/expired subscriptions (HTTP 410)
        const status = (err as { statusCode?: number }).statusCode;
        if (status === 410 || status === 404) {
          subscriptions.delete(sub.endpoint);
        }
      }
    })
  );

  return NextResponse.json({ sent, failed });
}
