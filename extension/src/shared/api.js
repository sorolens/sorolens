// Minimal Sorolens REST client (fetch only, no SDK runtime dependency).
//
// Endpoints and payloads mirror docs/openapi.yaml and the live routes in
// apps/api/internal/router/router.go:
//
//   GET    /health                                  liveness probe
//   GET    /api/v1/contracts/{id}                   is the contract indexed?
//   POST   /api/v1/contracts                        register for tracking
//   GET    /api/v1/watchlist/{contractId}/status    per-user tracking status
//   POST   /api/v1/watchlist                        add to the user's watchlist
//   DELETE /api/v1/watchlist/{contractId}           remove from the watchlist
//   GET    /api/v1/watchlist                        list the user's watchlist
//
// Authentication:
//   Authorization: Bearer <API key>   the credential; a user-bound key acts as
//                                      that user, which is what the /watchlist
//                                      routes key on
//   X-User-ID: <identity>            sent, but ignored unless a trusted proxy
//                                      signed it (issue #518)
//
// A key is minted by the dashboard (Settings -> Browser extension) with the
// read:contracts and write:contracts scopes. Reads work with no credential at
// all on the public v0.1 surface; watchlist writes answer 401 without one.

import { REQUEST_TIMEOUT_MS } from "./constants.js";

export class SorolensApiError extends Error {
  /**
   * @param {string} message
   * @param {{ status?: number, code?: string, url?: string, retryAfter?: number }} [details]
   */
  constructor(message, { status = 0, code = "", url = "", retryAfter = 0 } = {}) {
    super(message);
    this.name = "SorolensApiError";
    this.status = status;
    this.code = code;
    this.url = url;
    /** Seconds requested by Retry-After, or 0 when absent/invalid. */
    this.retryAfter = retryAfter;
  }
}

/**
 * @typedef {object} TrackingStatus
 * @property {string} contractId
 * @property {boolean} tracked            registered for indexing (network-wide)
 * @property {boolean} inWatchlist        on this user's watchlist
 * @property {string} [status]            contract status when tracked
 * @property {string} [error]             set when a request failed
 */

/**
 * @typedef {object} TrackOutcome
 * @property {string} contractId
 * @property {boolean} registered
 * @property {boolean} inWatchlist
 * @property {string[]} warnings
 */

export class SorolensClient {
  /**
   * @param {object} config
   * @param {string} config.apiBaseUrl
   * @param {string} [config.apiKey]
   * @param {string} [config.userId]
   * @param {string} [config.network]
   * @param {typeof fetch} [config.fetchImpl]
   * @param {number} [config.timeoutMs]
   */
  constructor({
    apiBaseUrl,
    apiKey = "",
    userId = "",
    network = "",
    fetchImpl = globalThis.fetch,
    timeoutMs = REQUEST_TIMEOUT_MS,
  }) {
    if (typeof fetchImpl !== "function") {
      throw new SorolensApiError("No fetch implementation available");
    }
    this.apiBaseUrl = String(apiBaseUrl || "").replace(/\/+$/, "");
    this.apiKey = apiKey;
    this.userId = userId;
    this.network = network;
    this.fetchImpl = fetchImpl;
    this.timeoutMs = timeoutMs;
  }

  /** True when requests can be authenticated with a scoped API key. */
  get hasApiKey() {
    return Boolean(this.apiKey);
  }

  /**
   * @param {string} path
   * @param {Record<string, string | undefined>} [params]
   */
  buildUrl(path, params) {
    const url = new URL(`${this.apiBaseUrl}${path}`);
    for (const [key, value] of Object.entries(params || {})) {
      if (value !== undefined && value !== null && value !== "") {
        url.searchParams.set(key, String(value));
      }
    }
    return url.toString();
  }

  /** @returns {Record<string, string>} */
  buildHeaders(hasBody) {
    const headers = { Accept: "application/json" };
    if (hasBody) headers["Content-Type"] = "application/json";
    if (this.apiKey) headers.Authorization = `Bearer ${this.apiKey}`;
    if (this.userId) headers["X-User-ID"] = this.userId;
    return headers;
  }

  /**
   * @param {string} method
   * @param {string} path
   * @param {{ body?: unknown, params?: Record<string, string | undefined> }} [options]
   */
  async request(method, path, { body, params } = {}) {
    const url = this.buildUrl(path, params);
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), this.timeoutMs);
    try {
      const response = await this.fetchImpl(url, {
        method,
        headers: this.buildHeaders(body !== undefined),
        body: body === undefined ? undefined : JSON.stringify(body),
        signal: controller.signal,
        cache: "no-store",
        credentials: "omit",
      });
      const payload = await readJson(response);
      if (!response.ok) {
        throw new SorolensApiError(
          (payload && payload.error) || response.statusText || "Request failed",
          {
            status: response.status,
            code: (payload && payload.code) || "",
            url,
            retryAfter: parseRetryAfter(
              response.headers && response.headers.get
                ? response.headers.get("Retry-After")
                : ""
            ),
          }
        );
      }
      return payload;
    } catch (error) {
      if (error && error.name === "AbortError") {
        throw new SorolensApiError(
          `Sorolens API timed out after ${this.timeoutMs}ms`,
          { status: 0, code: "timeout", url }
        );
      }
      throw error;
    } finally {
      clearTimeout(timer);
    }
  }

  /** `GET /health` */
  async health() {
    return this.request("GET", "/health");
  }

  /**
   * `GET /api/v1/contracts/{id}` — `null` when the API answers 404.
   * @param {string} contractId
   */
  async getContract(contractId) {
    try {
      return await this.request(
        "GET",
        `/api/v1/contracts/${encodeURIComponent(contractId)}`
      );
    } catch (error) {
      if (error instanceof SorolensApiError && error.status === 404) {
        return null;
      }
      throw error;
    }
  }

  /**
   * `POST /api/v1/contracts` — requires `write:contracts` and contributor role.
   * @param {{ id: string, network?: string, label?: string }} input
   */
  async registerContract({ id, network, label }) {
    return this.request("POST", "/api/v1/contracts", {
      body: {
        id,
        network: network || this.network || "mainnet",
        ...(label ? { label } : {}),
      },
    });
  }

  /**
   * `GET /api/v1/watchlist/{contractId}/status`
   * @param {string} contractId
   * @returns {Promise<boolean>}
   */
  async watchlistStatus(contractId) {
    const payload = await this.request(
      "GET",
      `/api/v1/watchlist/${encodeURIComponent(contractId)}/status`
    );
    return Boolean(payload && payload.in_watchlist);
  }

  /**
   * `POST /api/v1/watchlist`
   * @param {string} contractId
   * @returns {Promise<boolean>}
   */
  async addToWatchlist(contractId) {
    const payload = await this.request("POST", "/api/v1/watchlist", {
      body: { contract_id: contractId },
    });
    return Boolean(payload && payload.in_watchlist);
  }

  /**
   * `DELETE /api/v1/watchlist/{contractId}`
   * @param {string} contractId
   * @returns {Promise<boolean>}
   */
  async removeFromWatchlist(contractId) {
    const payload = await this.request(
      "DELETE",
      `/api/v1/watchlist/${encodeURIComponent(contractId)}`
    );
    return Boolean(payload && payload.in_watchlist);
  }

  /** `GET /api/v1/watchlist` */
  async listWatchlist() {
    const payload = await this.request("GET", "/api/v1/watchlist");
    const items = (payload && payload.items) || [];
    return items.map((item) => item.contract_id).filter(Boolean);
  }

  /**
   * `GET /api/v1/alerts?flat=true` — the raw alert feed, newest first.
   * Optionally scoped to one contract.
   *
   * @param {{ contractId?: string, limit?: number }} [options]
   * @returns {Promise<Array<Record<string, any>>>}
   */
  async listAlertsFlat({ contractId = "", limit = 100 } = {}) {
    const payload = await this.request("GET", "/api/v1/alerts", {
      params: {
        flat: "true",
        limit: String(limit),
        ...(contractId ? { contract_id: contractId } : {}),
      },
    });
    const alerts = (payload && payload.alerts) || [];
    return Array.isArray(alerts) ? alerts : [];
  }

  /**
   * Reads both halves of "is this contract being watched?" in one round trip.
   * A failure on one side is reported in `error` instead of rejecting, so the
   * popup can render partial state.
   *
   * @param {string} contractId
   * @returns {Promise<TrackingStatus>}
   */
  async getTrackingStatus(contractId) {
    const [contract, watchlist] = await Promise.allSettled([
      this.getContract(contractId),
      this.watchlistStatus(contractId),
    ]);
    const errors = [];
    if (contract.status === "rejected") {
      errors.push(describeError(contract.reason));
    }
    if (watchlist.status === "rejected") {
      errors.push(describeError(watchlist.reason));
    }
    const result = {
      contractId,
      tracked: contract.status === "fulfilled" && Boolean(contract.value),
      inWatchlist: watchlist.status === "fulfilled" && Boolean(watchlist.value),
    };
    if (
      contract.status === "fulfilled" &&
      contract.value &&
      contract.value.status
    ) {
      result.status = contract.value.status;
    }
    if (errors.length > 0) {
      result.error = errors.join("; ");
    }
    return result;
  }

  /**
   * One-click tracking: make sure the contract is registered for indexing, then
   * add it to this user's watchlist. Registration can legitimately fail (the
   * identity is not a contributor, or the key lacks `write:contracts`) without
   * blocking the watchlist add, so that failure is surfaced as a warning.
   *
   * @param {string} contractId
   * @param {{ network?: string, label?: string, watchlist?: boolean }} [options]
   * @returns {Promise<TrackOutcome>}
   */
  async track(contractId, { network, label, watchlist = true } = {}) {
    const warnings = [];
    let registered = false;

    const existing = await this.getContract(contractId);
    if (existing) {
      registered = true;
    } else {
      try {
        await this.registerContract({ id: contractId, network, label });
        registered = true;
      } catch (error) {
        warnings.push(
          `Contract is not indexed yet: ${describeError(error)}. ` +
            "Registration needs an API key with write:contracts and a contributor identity."
        );
      }
    }

    let inWatchlist = false;
    if (watchlist) {
      try {
        inWatchlist = await this.addToWatchlist(contractId);
      } catch (error) {
        warnings.push(`Watchlist: ${describeError(error)}`);
      }
    }

    return { contractId, registered, inWatchlist, warnings };
  }

  /**
   * @param {string} contractId
   * @returns {Promise<{ contractId: string, inWatchlist: boolean }>}
   */
  async untrack(contractId) {
    const inWatchlist = await this.removeFromWatchlist(contractId);
    return { contractId, inWatchlist };
  }
}

/**
 * Parses a Retry-After header (seconds or an HTTP date) into seconds. Returns 0
 * when the value is missing or unparseable, so callers can fall back to their
 * own backoff.
 * @param {string} value
 */
export function parseRetryAfter(value) {
  if (!value) return 0;
  const seconds = Number(value);
  if (Number.isFinite(seconds) && seconds > 0) return Math.ceil(seconds);
  const at = Date.parse(value);
  if (Number.isNaN(at)) return 0;
  return Math.max(0, Math.ceil((at - Date.now()) / 1000));
}

/** Reads a JSON body without throwing on empty/invalid payloads. */
async function readJson(response) {
  try {
    const text = await response.text();
    if (!text) return null;
    return JSON.parse(text);
  } catch {
    return null;
  }
}

/** @param {unknown} error */
export function describeError(error) {
  if (error instanceof SorolensApiError) {
    if (error.status === 401) return "API key missing, invalid or revoked";
    if (error.status === 403) {
      return `${error.message} (required scope or role not granted)`;
    }
    if (error.status === 429) return "Rate limited by the Sorolens API";
    if (error.status >= 500) return "Sorolens API error";
    return error.message;
  }
  if (error instanceof Error) return error.message;
  return String(error);
}
