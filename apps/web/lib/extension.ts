/**
 * Where the browser extension lives and what it needs to work (issue #764).
 *
 * The extension is not published to the Chrome Web Store or AMO yet, so the
 * install path is the repository directory (load unpacked) or a release build.
 * Both links are absolute because they point outside this app; the canonical
 * origin for our own URLs comes from `lib/siteUrl`.
 */
export const EXTENSION_INSTALL_URL =
  "https://github.com/sorolens/sorolens/tree/main/extension";

export const EXTENSION_RELEASES_URL =
  "https://github.com/sorolens/sorolens/releases/latest";

/** Name given to a key minted for the extension, so it is recognisable in the API-keys list. */
export const EXTENSION_KEY_NAME = "Browser extension";

/**
 * What the extension actually calls: `GET /api/v1/contracts/{id}` to check
 * whether a contract is indexed, `POST /api/v1/contracts` to register one, and
 * the `/watchlist` routes to add or remove it. Only the first two are
 * scope-gated (`apps/api/internal/middleware/scopes.go`); the watchlist routes
 * and `GET /api/v1/alerts` are unscoped and key off the identity the bound key
 * supplies. `read:watchdog` is deliberately absent — the extension polls the
 * alert feed, not the watchdog endpoints.
 */
export const EXTENSION_KEY_SCOPES: readonly string[] = [
  "read:contracts",
  "write:contracts",
];
