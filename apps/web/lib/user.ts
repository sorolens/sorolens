/**
 * Shared browser identity helper.
 *
 * The API's write routes (track contract, tag contract) require a recognized
 * contributor identity. Until auth is wired up, the dashboard stores an
 * operator-supplied user ID in localStorage and forwards it as X-User-ID,
 * matching the convention the watchlist page already uses.
 */

export const USER_ID_STORAGE_KEY = "sorolens_user_id";

/** Returns the stored user ID, or "" when none is available. */
export function getUserId(): string {
  if (typeof window === "undefined") return "";
  try {
    return window.localStorage.getItem(USER_ID_STORAGE_KEY) || "";
  } catch {
    // localStorage can be unavailable (private mode, some test runners);
    // reads remain open for anonymous callers, only writes need identity.
    return "";
  }
}
