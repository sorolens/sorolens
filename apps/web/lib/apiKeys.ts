/**
 * Real API-key calls for the `/settings` page (issue #613).
 *
 * Before this module the page "generated" a random `sl_live_...` string and
 * stored it in localStorage: nothing on the server knew about it, so the UI
 * implied a credential that authenticates nothing. Key creation now goes
 * through POST /api/v1/api-keys and revocation through
 * DELETE /api/v1/api-keys/{id}. The plaintext token is returned once by the
 * API (only its SHA-256 hash is stored server-side); we keep it in component
 * memory for the one-time copy and never persist it.
 *
 * The endpoints require admin scope/role (see apps/api/internal/router). A
 * minted browser session is contributor-only, so the honest outcome from this
 * card may be a 401/403 — the error text from the API is surfaced verbatim
 * rather than papered over with a fake key (issue #613).
 */

import { API_URL } from "./config";
import { ApiError } from "./api";
import { sessionHeaders } from "./user";

/** The subset of the APIKey response the settings card needs. */
export interface CreatedApiKey {
  id: string;
  name: string;
  key_prefix: string;
  created_at: string;
  /** Plaintext token; shown once by the API and never re-fetchable. */
  key: string;
}

interface ApiKeyCreatePayload {
  id: string;
  name: string;
  key_prefix: string;
  created_at: string;
  key?: string;
}

async function parseError(res: Response): Promise<never> {
  let body: { error?: string; code?: string } = {};
  try {
    body = (await res.json()) as { error?: string; code?: string };
  } catch {
    // Non-JSON error body: fall through to the status text.
  }
  throw new ApiError(res.status, body.error || res.statusText, body.code);
}

/**
 * Creates a key on the API for the given session user. Throws ApiError with
 * the server's own message when the caller lacks admin scope, the body is
 * invalid, or the API is unreachable.
 */
export async function createApiKey(opts: {
  name: string;
  scopes: string[];
  userId?: string;
}): Promise<CreatedApiKey> {
  const res = await fetch(`${API_URL}/api/v1/api-keys`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...sessionHeaders(opts.userId),
    },
    body: JSON.stringify({ name: opts.name, scopes: opts.scopes }),
  });
  if (!res.ok) await parseError(res);
  const payload = (await res.json()) as ApiKeyCreatePayload;
  if (!payload.id || !payload.key) {
    throw new ApiError(502, "API returned an incomplete key response");
  }
  return {
    id: payload.id,
    name: payload.name,
    key_prefix: payload.key_prefix,
    created_at: payload.created_at,
    key: payload.key,
  };
}

/**
 * Revokes a key on the API. The endpoint answers 204 with no body, so any
 * non-OK response is turned into an ApiError carrying the server message.
 */
export async function revokeApiKey(id: string, userId?: string): Promise<void> {
  const res = await fetch(
    `${API_URL}/api/v1/api-keys/${encodeURIComponent(id)}`,
    { method: "DELETE", headers: sessionHeaders(userId) }
  );
  if (!res.ok) await parseError(res);
}
