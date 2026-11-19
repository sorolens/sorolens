/**
 * Webhook verification built on Web Crypto (`crypto.subtle`), which is present
 * in browsers, Cloudflare Workers, Deno, Bun and Node 18+. No Node `crypto`
 * import is used, so this module is safe in an edge function.
 *
 * Scheme (see `docs/webhooks.md`): signed payload is `timestamp + "." + body`,
 * HMAC-SHA256 keyed with the full `whsec_...` secret, hex encoded. Any `v1`
 * signature may match, comparison is constant time, and the timestamp must be
 * within the replay window.
 */

export interface VerifyWebhookOptions {
  /** The subscription signing secret, including the `whsec_` prefix. */
  secret: string;
  /** The `X-Sorolens-Signature` header (`t=...,v1=...`). */
  signature: string | null | undefined;
  /** The `X-Sorolens-Timestamp` header. */
  timestamp: string | null | undefined;
  /** The exact raw request body, before JSON parsing. */
  body: string | Uint8Array;
  /** Replay window in seconds. Default 300 (five minutes). */
  toleranceSeconds?: number;
  /** Current Unix time in seconds. Default `Date.now() / 1000`. */
  nowSeconds?: number;
}

export const DEFAULT_TOLERANCE_SECONDS = 300;

const encoder = new TextEncoder();

export async function verifyWebhook(options: VerifyWebhookOptions): Promise<boolean> {
  const {
    secret,
    signature,
    timestamp,
    body,
    toleranceSeconds = DEFAULT_TOLERANCE_SECONDS,
    nowSeconds = Math.floor(Date.now() / 1000),
  } = options;

  if (!secret || !signature || !timestamp) return false;

  let headerTimestamp: string | undefined;
  const signatures: string[] = [];
  for (const part of signature.split(",")) {
    const eq = part.indexOf("=");
    if (eq <= 0) continue;
    const key = part.slice(0, eq).trim();
    const value = part.slice(eq + 1).trim();
    if (key === "t") headerTimestamp = value;
    else if (key === "v1" && value) signatures.push(value);
  }

  if (!headerTimestamp || signatures.length === 0) return false;
  if (headerTimestamp !== timestamp.trim()) return false;

  const ts = Number(headerTimestamp);
  if (!Number.isFinite(ts)) return false;
  if (Math.abs(nowSeconds - ts) > toleranceSeconds) return false;

  const bodyBytes = typeof body === "string" ? encoder.encode(body) : body;
  const prefix = encoder.encode(`${headerTimestamp}.`);
  const payload = new Uint8Array(prefix.length + bodyBytes.length);
  payload.set(prefix, 0);
  payload.set(bodyBytes, prefix.length);

  const key = await crypto.subtle.importKey(
    "raw",
    encoder.encode(secret),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"],
  );
  const expected = new Uint8Array(await crypto.subtle.sign("HMAC", key, payload));

  let matched = false;
  for (const candidate of signatures) {
    const bytes = hexToBytes(candidate);
    if (bytes && equalInConstantTime(bytes, expected)) {
      matched = true;
    }
  }
  return matched;
}

/** True when the two byte arrays are equal, without early exit. */
export function equalInConstantTime(a: Uint8Array, b: Uint8Array): boolean {
  if (a.length !== b.length) return false;
  let diff = 0;
  for (let i = 0; i < a.length; i += 1) {
    diff |= (a[i] as number) ^ (b[i] as number);
  }
  return diff === 0;
}

function hexToBytes(hex: string): Uint8Array | undefined {
  if (hex.length % 2 !== 0 || !/^[0-9a-fA-F]+$/.test(hex)) return undefined;
  const bytes = new Uint8Array(hex.length / 2);
  for (let i = 0; i < bytes.length; i += 1) {
    bytes[i] = Number.parseInt(hex.slice(i * 2, i * 2 + 2), 16);
  }
  return bytes;
}
