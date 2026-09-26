// Message contract between the content script, the popup and the service
// worker. Keeping the type strings in one module stops the three contexts from
// drifting apart, and gives the background router a single allow-list.

export const MESSAGE = {
  /** popup -> background: are we alive? */
  ping: "sorolens:ping",
  /** popup/content -> background: status for one contract. */
  status: "sorolens:status",
  /** popup/content -> background: status for many contracts. */
  statusBatch: "sorolens:status-batch",
  /** popup/content -> background: register + watchlist. */
  track: "sorolens:track",
  /** popup -> background: remove from the watchlist. */
  untrack: "sorolens:untrack",
  /** popup/options -> background: read settings (never returns the key). */
  getSettings: "sorolens:get-settings",
  /** options -> background: persist a settings patch. */
  saveSettings: "sorolens:save-settings",
  /** options -> background: `GET /health` + status cache reset. */
  testConnection: "sorolens:test-connection",
  /** content -> background: IDs found on the current page. */
  scanResult: "sorolens:scan-result",
  /** popup -> background: last scan reported for a tab. */
  getScan: "sorolens:get-scan",
  /** popup -> content script: contract IDs found on the tab. */
  getDetected: "sorolens:get-detected",
  /** background -> content script: re-scan the page (settings changed). */
  rescan: "sorolens:rescan",
};

export const MESSAGE_TYPES = Object.values(MESSAGE);

/**
 * @typedef {object} SorolensMessage
 * @property {string} type
 * @property {string} [contractId]
 * @property {string[]} [contractIds]
 * @property {string} [network]
 * @property {string} [label]
 * @property {number} [tabId]
 * @property {string} [url]
 * @property {boolean} [force]
 * @property {Record<string, unknown>} [patch]
 */

/** @param {unknown} value */
export function isSorolensMessage(value) {
  return Boolean(
    value &&
      typeof value === "object" &&
      typeof (/** @type {SorolensMessage} */ (value).type) === "string" &&
      MESSAGE_TYPES.includes(/** @type {SorolensMessage} */ (value).type),
  );
}

/**
 * The API key is sent to the options page only after an explicit reveal, so
 * `get-settings` responses mask it by default.
 * @param {import("./storage.js").SorolensSettings} settings
 */
export function maskApiKey(apiKey) {
  if (!apiKey) return "";
  if (apiKey.length <= 4) return "•".repeat(apiKey.length);
  return `${"•".repeat(Math.min(apiKey.length - 4, 20))}${apiKey.slice(-4)}`;
}
