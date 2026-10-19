// Extension settings and cached tracking status.
//
// Layout:
//   storage.local  -> the API key (secret, stays on this device) + status cache
//   storage.sync   -> non-secret preferences (API base URL, network, identity)
//
// Splitting the two means a Chrome profile sync never copies the API key to
// another machine, while the network/base-URL preferences still follow the user.

import { storageGet, storageSet, storageRemove } from "./browser.js";
import {
  DEFAULT_API_BASE_URL,
  DEFAULT_NETWORK,
  LOCAL_KEYS,
  NETWORKS,
  STATUS_CACHE_MAX_ENTRIES,
  STATUS_CACHE_TTL_MS,
  SYNC_KEYS,
} from "./constants.js";

/**
 * @typedef {object} SorolensSettings
 * @property {string} apiKey
 * @property {string} apiBaseUrl
 * @property {string} network
 * @property {string} userId
 * @property {boolean} showInlineButtons
 * @property {boolean} trackWatchlist
 */

/** @type {SorolensSettings} */
export const DEFAULT_SETTINGS = {
  apiKey: "",
  apiBaseUrl: DEFAULT_API_BASE_URL,
  network: DEFAULT_NETWORK,
  userId: "",
  showInlineButtons: true,
  trackWatchlist: true,
};

/** Trims a user-entered base URL and drops a trailing slash. */
export function normalizeBaseUrl(value) {
  if (typeof value !== "string") return "";
  const trimmed = value.trim().replace(/\/+$/, "");
  if (!trimmed) return "";
  return trimmed;
}

/** `https://api.sorolens.xyz` -> `https://api.sorolens.xyz/*` */
export function baseUrlToOriginPattern(baseUrl) {
  const normalized = normalizeBaseUrl(baseUrl);
  if (!normalized) return "";
  try {
    const url = new URL(normalized);
    if (url.protocol !== "http:" && url.protocol !== "https:") return "";
    return `${url.origin}/*`;
  } catch {
    return "";
  }
}

/** Dashboard identity format, mirrored from apps/web (`sorolens_user_id`). */
export function generateUserId() {
  const random = Math.random().toString(36).slice(2, 10);
  return `user_${Date.now()}_${random}`;
}

/** @returns {Promise<SorolensSettings>} */
export async function getSettings() {
  const [secrets, prefs] = await Promise.all([
    storageGet("local", { [LOCAL_KEYS.apiKey]: "" }),
    storageGet("sync", {
      [SYNC_KEYS.apiBaseUrl]: DEFAULT_API_BASE_URL,
      [SYNC_KEYS.network]: DEFAULT_NETWORK,
      [SYNC_KEYS.userId]: "",
      [SYNC_KEYS.showInlineButtons]: true,
      [SYNC_KEYS.trackWatchlist]: true,
    }),
  ]);
  const network = String(prefs[SYNC_KEYS.network] || DEFAULT_NETWORK);
  return {
    apiKey: String(secrets[LOCAL_KEYS.apiKey] || ""),
    apiBaseUrl:
      normalizeBaseUrl(String(prefs[SYNC_KEYS.apiBaseUrl] || "")) ||
      DEFAULT_API_BASE_URL,
    network: NETWORKS.includes(network) ? network : DEFAULT_NETWORK,
    userId: String(prefs[SYNC_KEYS.userId] || ""),
    showInlineButtons: prefs[SYNC_KEYS.showInlineButtons] !== false,
    trackWatchlist: prefs[SYNC_KEYS.trackWatchlist] !== false,
  };
}

/**
 * Applies a partial settings patch, routing secrets and prefs to their area.
 * @param {Partial<SorolensSettings>} patch
 * @returns {Promise<SorolensSettings>}
 */
export async function saveSettings(patch) {
  const local = {};
  const sync = {};
  if (typeof patch.apiKey === "string") {
    local[LOCAL_KEYS.apiKey] = patch.apiKey.trim();
  }
  if (patch.apiBaseUrl !== undefined) {
    sync[SYNC_KEYS.apiBaseUrl] =
      normalizeBaseUrl(patch.apiBaseUrl) || DEFAULT_API_BASE_URL;
  }
  if (patch.network !== undefined) {
    sync[SYNC_KEYS.network] = NETWORKS.includes(patch.network)
      ? patch.network
      : DEFAULT_NETWORK;
  }
  if (patch.userId !== undefined) {
    sync[SYNC_KEYS.userId] = patch.userId.trim() || generateUserId();
  }
  if (patch.showInlineButtons !== undefined) {
    sync[SYNC_KEYS.showInlineButtons] = Boolean(patch.showInlineButtons);
  }
  if (patch.trackWatchlist !== undefined) {
    sync[SYNC_KEYS.trackWatchlist] = Boolean(patch.trackWatchlist);
  }
  const writes = [];
  if (Object.keys(local).length > 0) writes.push(storageSet("local", local));
  if (Object.keys(sync).length > 0) writes.push(storageSet("sync", sync));
  await Promise.all(writes);
  return getSettings();
}

/**
 * Returns the stored identity, creating and persisting one on first use.
 * @returns {Promise<string>}
 */
export async function ensureUserId() {
  const { userId } = await getSettings();
  if (userId) return userId;
  const generated = generateUserId();
  await storageSet("sync", { [SYNC_KEYS.userId]: generated });
  return generated;
}

/** Clears the stored API key without touching the other preferences. */
export async function clearApiKey() {
  await storageRemove("local", LOCAL_KEYS.apiKey);
}

/**
 * @typedef {object} CachedStatus
 * @property {string} contractId
 * @property {boolean} tracked
 * @property {boolean} inWatchlist
 * @property {number} checkedAt
 * @property {string} [error]
 */

/** @returns {Promise<Record<string, CachedStatus>>} */
export async function readStatusCache() {
  const stored = await storageGet("local", { [LOCAL_KEYS.statusCache]: {} });
  const cache = stored[LOCAL_KEYS.statusCache];
  return cache && typeof cache === "object" ? cache : {};
}

/** Returns the cached entry when it is still inside the TTL window. */
export function readFreshEntry(cache, contractId, now = Date.now()) {
  const entry = cache[contractId];
  if (!entry || typeof entry.checkedAt !== "number") return null;
  if (now - entry.checkedAt > STATUS_CACHE_TTL_MS) return null;
  return entry;
}

/**
 * Merges fresh entries into the cache, dropping the oldest beyond the cap.
 * @param {Record<string, CachedStatus>} cache
 * @param {CachedStatus[]} entries
 */
export async function writeStatusCache(cache, entries) {
  const now = Date.now();
  const next = { ...cache };
  for (const entry of entries) {
    const checkedAt =
      typeof entry.checkedAt === "number" ? entry.checkedAt : now;
    next[entry.contractId] = { ...entry, checkedAt };
  }
  const keys = Object.keys(next);
  if (keys.length > STATUS_CACHE_MAX_ENTRIES) {
    keys
      .sort((a, b) => (next[a].checkedAt || 0) - (next[b].checkedAt || 0))
      .slice(0, keys.length - STATUS_CACHE_MAX_ENTRIES)
      .forEach((key) => {
        delete next[key];
      });
  }
  await storageSet("local", { [LOCAL_KEYS.statusCache]: next });
  return next;
}
