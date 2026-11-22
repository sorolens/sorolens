/**
 * Browser-side user identity.
 *
 * The API no longer trusts identity headers (issue #518). Instead the dashboard
 * mints a workspace session with `POST /api/v1/auth/session` and sends the
 * returned HMAC-signed token as `Authorization: Bearer sls_...`. The session
 * carries the user ID and the contributor role, so writes work without any
 * user provisioning step.
 */

import { API_URL } from "./config";

/** localStorage key holding the minted session. */
export const SESSION_STORAGE_KEY = "sorolens_session";

/** Legacy self-asserted identity key; removed once a session is minted. */
const LEGACY_USER_STORAGE_KEY = "sorolens_user_id";

export interface StoredSession {
  user_id: string;
  token: string;
  expires_at: string;
}

function isStoredSession(value: unknown): value is StoredSession {
  if (!value || typeof value !== "object") return false;
  const s = value as Record<string, unknown>;
  return (
    typeof s.user_id === "string" &&
    s.user_id !== "" &&
    typeof s.token === "string" &&
    s.token.startsWith("sls_") &&
    typeof s.expires_at === "string"
  );
}

/**
 * Returns the stored session while it is still valid, or null. A small skew is
 * required so a token is not sent moments before it expires.
 */
export function getSession(): StoredSession | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = window.localStorage.getItem(SESSION_STORAGE_KEY);
    if (!raw) return null;
    const parsed: unknown = JSON.parse(raw);
    if (!isStoredSession(parsed)) return null;
    if (Date.parse(parsed.expires_at) - 60_000 <= Date.now()) return null;
    return parsed;
  } catch {
    return null;
  }
}

/** Reads the current user ID from the session, or `""` when there is none. */
export function getUserId(): string {
  return getSession()?.user_id ?? "";
}

/**
 * Builds the authorization headers for a call made on behalf of userId. If the
 * stored session belongs to a different user the header is omitted, which
 * surfaces as a 401 instead of acting as the wrong user.
 */
export function sessionHeaders(userId?: string): Record<string, string> {
  if (!userId) return {};
  const session = getSession();
  if (!session || session.user_id !== userId) return {};
  return { Authorization: `Bearer ${session.token}` };
}

let pending: Promise<StoredSession> | null = null;

/**
 * Returns the current session, minting one if none exists. Concurrent callers
 * share a single request so a page with several identity-scoped fetches does
 * not create multiple users.
 */
export function ensureSession(): Promise<StoredSession> {
  const existing = getSession();
  if (existing) return Promise.resolve(existing);
  if (!pending) {
    pending = mintSession().finally(() => {
      pending = null;
    });
  }
  return pending;
}

async function mintSession(): Promise<StoredSession> {
  const res = await fetch(`${API_URL}/api/v1/auth/session`, { method: "POST" });
  if (!res.ok) {
    throw new Error(
      res.status === 503
        ? "Workspace sessions are not configured on this API."
        : `Could not create a workspace session (${res.status}).`
    );
  }
  const body: unknown = await res.json();
  if (!isStoredSession(body)) {
    throw new Error("Workspace session response was malformed.");
  }
  if (typeof window !== "undefined") {
    try {
      window.localStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify(body));
      window.localStorage.removeItem(LEGACY_USER_STORAGE_KEY);
    } catch {
      // Private mode: the session still works for this page's lifetime.
    }
  }
  return body;
}
