// Shared test fixtures: an in-memory `chrome.*` implementation and a scripted
// fetch, so the extension modules can be exercised under plain Node.

export const CONTRACT_ID = "C" + "A".repeat(55);
export const OTHER_CONTRACT_ID = "C" + "B".repeat(55);
export const API_BASE = "https://api.sorolens.xyz";

/** Minimal fetch Response stand-in. */
export function jsonResponse(status, body) {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText: `HTTP ${status}`,
    text: async () => (body === undefined ? "" : JSON.stringify(body)),
  };
}

/**
 * Installs an in-memory `chrome` API on globalThis.
 * @returns {{ local: Map<string, unknown>, sync: Map<string, unknown>, restore: () => void }}
 */
export function installFakeBrowser() {
  const local = new Map();
  const sync = new Map();
  const previousChrome = globalThis.chrome;
  const previousBrowser = globalThis.browser;

  const area = (store) => ({
    get(defaults, callback) {
      const result = {};
      for (const [key, value] of Object.entries(defaults || {})) {
        result[key] = store.has(key) ? store.get(key) : value;
      }
      if (typeof callback === "function") callback(result);
      return undefined;
    },
    set(values, callback) {
      for (const [key, value] of Object.entries(values)) store.set(key, value);
      if (typeof callback === "function") callback();
      return undefined;
    },
    remove(keys, callback) {
      for (const key of [].concat(keys)) store.delete(key);
      if (typeof callback === "function") callback();
      return undefined;
    },
  });

  globalThis.chrome = {
    storage: {
      local: area(local),
      sync: area(sync),
      session: area(new Map()),
    },
    runtime: {
      lastError: null,
      getURL: (path) => `chrome-extension://test/${path}`,
      sendMessage: (_message, callback) => {
        if (typeof callback === "function") callback(undefined);
        return undefined;
      },
    },
    tabs: {
      query: (_query, callback) => {
        if (typeof callback === "function") callback([]);
        return undefined;
      },
      sendMessage: (_tabId, _message, callback) => {
        if (typeof callback === "function") callback(undefined);
        return undefined;
      },
    },
    permissions: {
      contains: (_permission, callback) => {
        if (typeof callback === "function") callback(true);
        return undefined;
      },
      request: (_permission, callback) => {
        if (typeof callback === "function") callback(true);
        return undefined;
      },
    },
    action: {
      setBadgeText: async () => undefined,
      setBadgeBackgroundColor: async () => undefined,
    },
  };

  return {
    local,
    sync,
    restore() {
      if (previousChrome === undefined) delete globalThis.chrome;
      else globalThis.chrome = previousChrome;
      if (previousBrowser === undefined) delete globalThis.browser;
      else globalThis.browser = previousBrowser;
    },
  };
}

/**
 * Installs a fetch implementation that routes requests through `handler`.
 * @param {(request: { method: string, url: URL, body: unknown }) => object} handler
 */
export function installFetch(handler) {
  const calls = [];
  const previous = globalThis.fetch;
  globalThis.fetch = async (url, options = {}) => {
    const parsed = new URL(url);
    const request = {
      method: options.method || "GET",
      url: parsed,
      body: options.body ? JSON.parse(options.body) : undefined,
      headers: options.headers || {},
    };
    calls.push(request);
    return handler(request);
  };
  return {
    calls,
    restore() {
      if (previous === undefined) delete globalThis.fetch;
      else globalThis.fetch = previous;
    },
    countFor(method, pathname) {
      return calls.filter(
        (call) => call.method === method && call.url.pathname === pathname,
      ).length;
    },
  };
}
