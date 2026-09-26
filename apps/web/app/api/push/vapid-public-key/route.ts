import { NextResponse } from "next/server";
import { getPublicVapidKey } from "@/lib/vapid";

/**
 * GET /api/push/vapid-public-key
 *
 * Returns the VAPID public key so the client-side code can call
 * PushManager.subscribe({ applicationServerKey }).
 *
 * The key is public by design — it is safe to expose.
 */
export async function GET() {
  try {
    const publicKey = getPublicVapidKey();
    return NextResponse.json({ publicKey });
  } catch (err) {
    const message =
      err instanceof Error ? err.message : "VAPID keys not configured";
    return NextResponse.json({ error: message }, { status: 503 });
  }
}
