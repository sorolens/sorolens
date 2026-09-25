import { NextRequest, NextResponse } from "next/server";
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
 * POST /api/push/subscribe
 * Body: PushSubscription JSON (endpoint + keys)
 *
 * Saves the subscription so the server can send push notifications later.
 */
export async function POST(request: NextRequest) {
  try {
    const body = (await request.json()) as PushSubscriptionJSON;

    if (!body?.endpoint || !body?.keys?.auth || !body?.keys?.p256dh) {
      return NextResponse.json(
        { error: "Invalid subscription payload" },
        { status: 400 }
      );
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
 * POST /api/push/send  (internal — called by the alerting pipeline, issue #127)
 *
 * When issue #127 is fully merged, the server-side alert delivery service
 * should call this endpoint (or use the vapid.ts helper directly) to fan out
 * push notifications to all subscribed clients.
 *
 * Accepts: { title, body, timestamp, contractId?, severity?, url? }
 * Returns: { sent: number, failed: number }
 *
 * This route is intentionally NOT public — protect it with an internal
 * shared secret (PUSH_INTERNAL_SECRET env var) before enabling in production.
 */
export async function PUT(request: NextRequest) {
  // Minimal internal auth guard
  const secret = request.headers.get("x-push-secret");
  const expected = process.env.PUSH_INTERNAL_SECRET;
  if (expected && secret !== expected) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const payload = await request.json();
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
