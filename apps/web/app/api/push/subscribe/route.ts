import { NextRequest, NextResponse } from "next/server";
import { API_URL } from "@/lib/config";
import { sendPushNotification, type PushSubscriptionJSON } from "@/lib/vapid";

/**
 * Web Push subscription routes (issue #592).
 *
 * These handlers used to keep subscriptions in an in-process Map that was
 * lost on every cold start. Persistence now lives in the Go API
 * (POST/GET/DELETE /api/v1/push/subscriptions, migration 000039): the API
 * stores endpoint + keys + user binding + filters, and re-subscribing the
 * same browser endpoint is idempotent there (UNIQUE endpoint, upsert).
 *
 * Server-to-API auth: when `SOROLENS_API_KEY` is configured the routes
 * present it as a bearer credential. Anonymous POST/DELETE still work
 * against the API (a push endpoint is an unguessable https URL owned by
 * the subscriber — same self-service posture as the email digest
 * subscriptions), but the fan-out list (used by the internal PUT sender
 * below) needs a contributor key with read:watchdog scope, so PUT answers
 * 503 without `SOROLENS_API_KEY`.
 *
 * Credential hygiene: the bearer key, subscriber endpoints and key
 * material are never written to responses or logs.
 */

const UPSTREAM_TIMEOUT_MS = 10_000;

function upstreamHeaders(): Record<string, string> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  const key = process.env.SOROLENS_API_KEY;
  if (key) headers.Authorization = `Bearer ${key}`;
  return headers;
}

/** Minimal shape check so a junk body never reaches the API; the API
 *  performs the authoritative RFC 8291 validation (https endpoint,
 *  base64url key material) and its 422 is surfaced as a 400 here. */
function validSubscriptionShape(body: unknown): body is PushSubscriptionJSON {
  const b = body as Partial<PushSubscriptionJSON>;
  return (
    !!b &&
    typeof b.endpoint === "string" &&
    b.endpoint.startsWith("https://") &&
    !!b.keys &&
    typeof b.keys.auth === "string" &&
    b.keys.auth.length > 0 &&
    typeof b.keys.p256dh === "string" &&
    b.keys.p256dh.length > 0
  );
}

async function upstreamError(res: Response): Promise<NextResponse> {
  // Surface the API's error message for validation failures; never echo
  // anything about our own credential.
  let message = "subscription service unavailable";
  try {
    const data = (await res.json()) as {
      error?: { message?: string };
      message?: string;
    };
    message = data.error?.message ?? data.message ?? message;
  } catch {
    // non-JSON upstream body: keep the generic message
  }
  const status = res.status === 422 ? 400 : res.status >= 500 ? 502 : res.status;
  return NextResponse.json({ ok: false, error: message }, { status });
}

/**
 * POST /api/push/subscribe
 * Body: PushSubscription JSON (endpoint + keys), optionally with user_id
 * and filters which the API understands.
 */
export async function POST(request: NextRequest) {
  let body: unknown;
  try {
    body = await request.json();
  } catch {
    return NextResponse.json(
      { ok: false, error: "Invalid subscription payload" },
      { status: 400 }
    );
  }

  if (!validSubscriptionShape(body)) {
    return NextResponse.json(
      { ok: false, error: "Invalid subscription payload" },
      { status: 400 }
    );
  }

  let res: Response;
  try {
    res = await fetch(`${API_URL}/api/v1/push/subscriptions`, {
      method: "POST",
      headers: upstreamHeaders(),
      body: JSON.stringify(body),
      signal: AbortSignal.timeout(UPSTREAM_TIMEOUT_MS),
    });
  } catch {
    return NextResponse.json(
      { ok: false, error: "subscription service unreachable" },
      { status: 502 }
    );
  }

  if (!res.ok) return upstreamError(res);

  const data = (await res.json()) as { id?: string };
  return NextResponse.json(
    { ok: true, id: data.id },
    { status: res.status === 201 ? 201 : 200 }
  );
}

/**
 * DELETE /api/push/subscribe
 * Body: { endpoint: string }  (browser unsubscribe flow)
 */
export async function DELETE(request: NextRequest) {
  let body: { endpoint?: string };
  try {
    body = (await request.json()) as { endpoint?: string };
  } catch {
    return NextResponse.json(
      { ok: false, error: "endpoint required" },
      { status: 400 }
    );
  }
  if (!body?.endpoint) {
    return NextResponse.json(
      { ok: false, error: "endpoint required" },
      { status: 400 }
    );
  }

  const url = `${API_URL}/api/v1/push/subscriptions?endpoint=${encodeURIComponent(
    body.endpoint
  )}`;
  let res: Response;
  try {
    res = await fetch(url, {
      method: "DELETE",
      headers: upstreamHeaders(),
      signal: AbortSignal.timeout(UPSTREAM_TIMEOUT_MS),
    });
  } catch {
    return NextResponse.json(
      { ok: false, error: "subscription service unreachable" },
      { status: 502 }
    );
  }

  if (res.status === 204) return NextResponse.json({ ok: true });
  if (!res.ok) return upstreamError(res);
  return NextResponse.json({ ok: true });
}

/**
 * PUT /api/push/subscribe  (internal — called by the alerting pipeline, issue #127)
 *
 * Fans out a payload to every stored subscription, read from the API
 * (contributor-scoped) rather than an in-process Map. Gone/expired
 * subscriptions (410/404 from the push service) are deleted through the API
 * so the persistent store self-heals.
 *
 * Accepts: { title, body, timestamp, contractId?, severity?, url? }
 * Returns: { sent: number, failed: number }
 *
 * This route is intentionally NOT public — protect it with an internal
 * shared secret (PUSH_INTERNAL_SECRET env var) before enabling in
 * production.
 */
export async function PUT(request: NextRequest) {
  const secret = request.headers.get("x-push-secret");
  const expected = process.env.PUSH_INTERNAL_SECRET;
  if (expected && secret !== expected) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  if (!process.env.SOROLENS_API_KEY) {
    return NextResponse.json(
      {
        error:
          "SOROLENS_API_KEY is not configured; the persistent subscription store is unreachable from this route",
      },
      { status: 503 }
    );
  }

  let listRes: Response;
  try {
    listRes = await fetch(`${API_URL}/api/v1/push/subscriptions`, {
      headers: upstreamHeaders(),
      signal: AbortSignal.timeout(UPSTREAM_TIMEOUT_MS),
    });
  } catch {
    return NextResponse.json(
      { error: "subscription service unreachable" },
      { status: 502 }
    );
  }
  if (!listRes.ok) {
    return NextResponse.json(
      { error: `subscription store returned status ${listRes.status}` },
      { status: 502 }
    );
  }
  const { subscriptions } = (await listRes.json()) as {
    subscriptions: Array<{
      endpoint: string;
      p256dh_key: string;
      auth_key: string;
    }>;
  };

  const payload = await request.json();
  let sent = 0;
  let failed = 0;

  await Promise.all(
    subscriptions.map(async (s) => {
      const sub: PushSubscriptionJSON = {
        endpoint: s.endpoint,
        keys: { auth: s.auth_key, p256dh: s.p256dh_key },
      };
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
        // Remove gone/expired subscriptions through the API (HTTP 410/404).
        const status = (err as { statusCode?: number }).statusCode;
        if (status === 410 || status === 404) {
          try {
            await fetch(
              `${API_URL}/api/v1/push/subscriptions?endpoint=${encodeURIComponent(
                s.endpoint
              )}`,
              {
                method: "DELETE",
                headers: upstreamHeaders(),
                signal: AbortSignal.timeout(UPSTREAM_TIMEOUT_MS),
              }
            );
          } catch {
            // self-heal is best effort; the next 410 retries it
          }
        }
      }
    })
  );

  return NextResponse.json({ sent, failed });
}
