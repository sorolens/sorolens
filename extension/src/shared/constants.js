// Shared constants for the Sorolens browser extension.
//
// Everything in this file is deliberately dependency-free so the same module
// can be imported by the MV3 service worker, the popup/options pages and the
// content-script bundle without a build step.

/** Production API base URL, taken from `servers[0].url` in docs/openapi.yaml. */
export const DEFAULT_API_BASE_URL = "https://api.sorolens.xyz";

/** Networks accepted by `POST /api/v1/contracts` (docs/openapi.yaml). */
export const NETWORKS = ["mainnet", "testnet", "futurenet", "standalone"];

export const DEFAULT_NETWORK = "mainnet";

/**
 * Soroban contract IDs are StrKey-encoded: a `C` version byte followed by 55
 * base32 characters (`A-Z2-7`), 56 characters total. The OpenAPI schema uses
 * the looser `^C[A-Za-z0-9]{55}$`; real IDs never contain `0`, `1`, `8`, `9`
 * or lowercase letters, so we validate the strict alphabet to avoid turning
 * random uppercase tokens into buttons.
 */
export const CONTRACT_ID_LENGTH = 56;
export const CONTRACT_ID_PREFIX = "C";
export const BASE32_ALPHABET = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";

export const CONTRACT_ID_HINT = "C" + "A".repeat(55);

/** Explorer pages the content script is allowed to run on. */
export const SUPPORTED_SITES = [
  "https://stellar.expert/*",
  "https://lab.stellar.org/*",
  "https://stellar.org/*",
];

/** Paths (relative to the extension root) referenced by the popup UI. */
export const PATHS = {
  options: "src/options/options.html",
  popup: "src/popup/popup.html",
};

/** `storage.local` keys. Secrets stay on-device and are never synced. */
export const LOCAL_KEYS = {
  apiKey: "sorolens.apiKey",
  statusCache: "sorolens.statusCache",
};

/** `storage.sync` keys. Non-secret preferences only. */
export const SYNC_KEYS = {
  apiBaseUrl: "sorolens.apiBaseUrl",
  network: "sorolens.network",
  userId: "sorolens.userId",
  showInlineButtons: "sorolens.showInlineButtons",
  trackWatchlist: "sorolens.trackWatchlist",
};

/** Where the dashboard keeps the identity it sends as `X-User-ID`. */
export const DASHBOARD_USER_ID_STORAGE_KEY = "sorolens_user_id";

/** How long a cached tracking status stays fresh, and how many we keep. */
export const STATUS_CACHE_TTL_MS = 60_000;
export const STATUS_CACHE_MAX_ENTRIES = 200;

/** Request timeout for API calls made from the service worker. */
export const REQUEST_TIMEOUT_MS = 10_000;

/** Content-script tuning. */
export const CONTENT = {
  /** Maximum inline buttons rendered per contract ID on one page. */
  maxButtonsPerId: 2,
  /** Maximum number of distinct contract IDs decorated on one page. */
  maxDecoratedIds: 25,
  /** Debounce for MutationObserver driven rescans. */
  rescanDebounceMs: 400,
};

export const BADGE = {
  /** Cyan from the dashboard logo (apps/web/public/logo.svg). */
  color: "#06B6D4",
};

export const PRODUCT_NAME = "Sorolens";
