// Thin `browser.*` shim so one codebase runs unchanged on Chrome and Firefox.
//
// Chrome exposes callback-style `chrome.*` APIs; Firefox exposes promise-style
// `browser.*` APIs (and keeps the `chrome.*` alias working). This module wraps
// the handful of calls the extension needs in a promise-first, callback-safe
// form. Nothing else in the extension touches `chrome`/`browser` directly.
//
// The API object is resolved lazily (not at module load) so unit tests can
// install a fake `globalThis.browser` before importing this module.

/**
 * @typedef {object} WebExtensionApi
 * @property {*} storage
 * @property {*} runtime
 * @property {*} tabs
 * @property {*} permissions
 */

/** @returns {WebExtensionApi} */
export function getBrowserApi() {
  const api = globalThis.browser || globalThis.chrome;
  if (!api) {
    throw new Error(
      "No WebExtension API found. The Sorolens extension must run inside Chrome or Firefox.",
    );
  }
  return api;
}

/** True when the promise-based `browser.*` namespace is available. */
export function isFirefox() {
  return typeof globalThis.browser !== "undefined";
}

/** @returns {*} the `lastError` reported by the last async call, if any. */
function lastError() {
  try {
    return getBrowserApi().runtime?.lastError ?? null;
  } catch {
    return null;
  }
}

/**
 * Runs an API call that supports both `(args..., callback)` and promise forms.
 * @template T
 * @param {(callback: (value: T) => void) => unknown} invoke
 * @returns {Promise<T>}
 */
function promisify(invoke) {
  return new Promise((resolve, reject) => {
    let settled = false;
    const done = (value) => {
      if (settled) return;
      settled = true;
      resolve(value);
    };
    const fail = (error) => {
      if (settled) return;
      settled = true;
      reject(error instanceof Error ? error : new Error(String(error)));
    };
    let maybePromise;
    try {
      maybePromise = invoke((value) => {
        const error = lastError();
        if (error) fail(new Error(error.message || "browser API error"));
        else done(value);
      });
    } catch (error) {
      fail(error);
      return;
    }
    if (maybePromise && typeof maybePromise.then === "function") {
      maybePromise.then(done, fail);
    }
  });
}

/** @returns {*} the storage area, or throws a descriptive error. */
function storageArea(area) {
  const store = getBrowserApi().storage?.[area];
  if (!store) {
    throw new Error(`storage.${area} is unavailable`);
  }
  return store;
}

/**
 * @param {"local" | "sync" | "session"} area
 * @param {Record<string, unknown>} defaults
 * @returns {Promise<Record<string, unknown>>}
 */
export function storageGet(area, defaults) {
  return promisify((callback) => storageArea(area).get(defaults, callback)).then(
    (items) => items || {},
  );
}

/**
 * @param {"local" | "sync" | "session"} area
 * @param {Record<string, unknown>} values
 * @returns {Promise<void>}
 */
export function storageSet(area, values) {
  return promisify((callback) => storageArea(area).set(values, callback)).then(
    () => undefined,
  );
}

/**
 * @param {"local" | "sync" | "session"} area
 * @param {string | string[]} keys
 * @returns {Promise<void>}
 */
export function storageRemove(area, keys) {
  return promisify((callback) => storageArea(area).remove(keys, callback)).then(
    () => undefined,
  );
}

/** Sends a message to the service worker. */
export function sendRuntimeMessage(message) {
  return promisify((callback) =>
    getBrowserApi().runtime.sendMessage(message, callback),
  );
}

/** Sends a message to a content script running in `tabId`. */
export function sendTabMessage(tabId, message) {
  return promisify((callback) =>
    getBrowserApi().tabs.sendMessage(tabId, message, callback),
  );
}

/** @returns {Promise<*>} the active tab in the current window. */
export async function getActiveTab() {
  const tabs = await promisify((callback) =>
    getBrowserApi().tabs.query({ active: true, currentWindow: true }, callback),
  );
  return Array.isArray(tabs) ? tabs[0] : undefined;
}

/**
 * Queries tabs. Sensitive fields (`url`, `title`) are only returned when the
 * extension holds the matching host permission.
 * @param {Record<string, unknown>} query
 * @returns {Promise<Array<{ id?: number, url?: string }>>}
 */
export async function queryTabs(query) {
  const tabs = await promisify((callback) =>
    getBrowserApi().tabs.query(query, callback),
  );
  return Array.isArray(tabs) ? tabs : [];
}

/** Opens the extension's options page. */
export function openOptionsPage() {
  return promisify((callback) => {
    const api = getBrowserApi();
    if (api.runtime.openOptionsPage) {
      return api.runtime.openOptionsPage(() => callback(undefined));
    }
    return api.tabs.create({ url: api.runtime.getURL("src/options/options.html") });
  });
}

/** `chrome.runtime.getURL` without touching `chrome` directly. */
export function runtimeUrl(path) {
  return getBrowserApi().runtime.getURL(path);
}

/**
 * Requests a host permission for a user-supplied API base URL.
 * Returns true when the permission is already granted or newly granted.
 * @param {string} origin like `https://api.example.com/*`
 */
export async function requestHostPermission(origin) {
  const api = getBrowserApi();
  const permissions = api.permissions;
  if (!permissions) {
    return false;
  }
  const contains = await promisify((callback) =>
    permissions.contains({ origins: [origin] }, callback),
  );
  if (contains) {
    return true;
  }
  return Boolean(
    await promisify((callback) =>
      permissions.request({ origins: [origin] }, callback),
    ),
  );
}
