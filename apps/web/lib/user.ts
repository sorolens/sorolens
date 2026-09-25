/**
 * Browser identity for the per-user features (watchlist, contract groups).
 *
 * The API scopes these resources by the `X-User-ID` header. In production this
 * would be the authenticated session user; the dashboard generates and stores a
 * stable local identity until auth lands.
 */
export const USER_STORAGE_KEY = "sorolens_user_id";

export function getUserId(): string {
  if (typeof window === "undefined") return "";
  try {
    let id = window.localStorage.getItem(USER_STORAGE_KEY);
    if (!id) {
      id = `user_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
      window.localStorage.setItem(USER_STORAGE_KEY, id);
    }
    return id;
  } catch {
    // localStorage can be unavailable (private mode, some test runners).
    return "";
  }
}
