/**
 * Client for the self-service API-key endpoints (issue #613).
 *
 * The `/api/v1/api-keys` routes are admin-only, so the dashboard cannot use
 * them: its session user is a contributor. These endpoints are bound to that
 * session user, and the plaintext token is returned exactly once at creation.
 * The token is never persisted; only the server-issued prefix is kept for
 * display.
 */

import { ApiError } from "./api";
import { API_URL } from "./config";
import { ensureSession } from "./user";

export interface APIKey {
  id: string;
  name: string;
  key_prefix: string;
  scopes: string[];
  user_id: string | null;
  created_at: string;
  last_used_at: string | null;
  revoked_at: string | null;
  rotated_at: string | null;
  /** CIDR ranges the key may be used from (issue #793). Empty = unrestricted. */
  allowed_cidrs: string[];
  /** Browser origins the key may be used from. Empty = unrestricted. */
  allowed_origins: string[];
}

export interface CreatedAPIKey extends APIKey {
  /** Plaintext token, present only in the create response. */
  key: string;
}

export const DEFAULT_KEY_SCOPES = ["read:contracts", "read:watchdog"];

async function authHeaders(): Promise<Record<string, string>> {
  const session = await ensureSession();
  return { Authorization: `Bearer ${session.token}` };
}

async function errorFrom(res: Response): Promise<ApiError> {
  let body: { error?: string; code?: string } = {};
  try {
    body = await res.json();
  } catch {
    // ignore parse error
  }
  return new ApiError(res.status, body.error || res.statusText, body.code);
}

export async function listMyApiKeys(): Promise<APIKey[]> {
  const res = await fetch(`${API_URL}/api/v1/me/api-keys`, {
    headers: await authHeaders(),
  });
  if (!res.ok) throw await errorFrom(res);
  const body = (await res.json()) as { api_keys?: APIKey[] };
  return body.api_keys ?? [];
}

export async function createMyApiKey(
  name = "Dashboard key",
  scopes: string[] = DEFAULT_KEY_SCOPES
): Promise<CreatedAPIKey> {
  const res = await fetch(`${API_URL}/api/v1/me/api-keys`, {
    method: "POST",
    headers: { ...(await authHeaders()), "Content-Type": "application/json" },
    body: JSON.stringify({ name, scopes }),
  });
  if (!res.ok) throw await errorFrom(res);
  return (await res.json()) as CreatedAPIKey;
}

export async function revokeMyApiKey(id: string): Promise<void> {
  const res = await fetch(
    `${API_URL}/api/v1/me/api-keys/${encodeURIComponent(id)}`,
    { method: "DELETE", headers: await authHeaders() }
  );
  if (!res.ok) throw await errorFrom(res);
}
