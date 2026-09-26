// MV3 service worker: the only context that talks to the Sorolens API.
//
// Content scripts and the popup are intentionally thin: they detect IDs and
// render UI, then send messages here. Keeping network + storage access in one
// place means the API key never has to be handed to a page-context script and
// the request/response shape is identical on Chrome and Firefox.

import { SorolensClient, describeError } from "../shared/api.js";
import { getBrowserApi } from "../shared/browser.js";
import { MESSAGE, isSorolensMessage } from "../shared/messages.js";
import {
  baseUrlToOriginPattern,
  ensureUserId,
  getSettings,
  readFreshEntry,
  readStatusCache,
  saveSettings,
  writeStatusCache,
} from "../shared/storage.js";
import { BADGE, CONTENT, DEFAULT_NETWORK, PRODUCT_NAME } from "../shared/constants.js";
import { normalizeContractId } from "../shared/contract-id.js";

/**
 * Latest contract IDs reported by the content script, per tab. In-memory is
 * enough: the popup asks the tab directly and only falls back here.
 * @type {Map<number, { contractIds: string[], url: string, at: number }>}
 */
const tabScans = new Map();

const CONTEXT_MENU_ID = "sorolens-track-selection";

/** Builds a client from the stored settings, creating the identity if needed. */
async function createClient() {
  const settings = await getSettings();
  const userId = settings.userId || (await ensureUserId());
  return {
    settings,
    client: new SorolensClient({
      apiBaseUrl: settings.apiBaseUrl,
      apiKey: settings.apiKey,
      userId,
      network: settings.network,
    }),
  };
}

/** @param {import("../shared/api.js").TrackingStatus} status */
function toCached(status) {
  return {
    contractId: status.contractId,
    tracked: status.tracked,
    inWatchlist: status.inWatchlist,
    checkedAt: Date.now(),
    ...(status.error ? { error: status.error } : {}),
  };
}

/**
 * Resolves statuses for a batch, serving fresh cache entries and fetching the
 * rest with a small concurrency limit so a page full of IDs cannot burst the
 * API's rate limit.
 *
 * @param {string[]} contractIds
 * @param {{ force?: boolean }} [options]
 */
async function resolveStatuses(contractIds, { force = false } = {}) {
  const { client, settings } = await createClient();
  const cache = await readStatusCache();
  const results = [];
  const missing = [];

  for (const contractId of contractIds) {
    const cached = force ? null : readFreshEntry(cache, contractId);
    if (cached) {
      results.push({
        contractId,
        tracked: cached.tracked,
        inWatchlist: cached.inWatchlist,
        ...(cached.error ? { error: cached.error } : {}),
      });
    } else {
      missing.push(contractId);
    }
  }

  const CONCURRENCY = 4;
  for (let i = 0; i < missing.length; i += CONCURRENCY) {
    const slice = missing.slice(i, i + CONCURRENCY);
    const settled = await Promise.all(
      slice.map((contractId) => client.getTrackingStatus(contractId)),
    );
    results.push(...settled);
  }

  if (results.length > 0) {
    await writeStatusCache(cache, results.map(toCached));
  }

  // Preserve the caller's ordering.
  const byId = new Map(results.map((status) => [status.contractId, status]));
  return contractIds
    .map((contractId) => byId.get(contractId))
    .filter(Boolean)
    .map((status) => ({ ...status, network: settings.network }));
}

/** Updates the toolbar badge to the number of IDs seen on the tab. */
async function refreshBadge(tabId, count) {
  try {
    const api = getBrowserApi();
    await api.action.setBadgeText({
      tabId,
      text: count > 0 ? String(count) : "",
    });
    if (count > 0) {
      await api.action.setBadgeBackgroundColor({ tabId, color: BADGE.color });
    }
  } catch {
    // Badge updates are best-effort; a closed tab must not break the flow.
  }
}

/** Seeds defaults, the context-menu entry and a stable identity on install. */
async function onInstalled(details) {
  await ensureUserId();
  const api = getBrowserApi();
  if (api.contextMenus && details && details.reason === "install") {
    api.contextMenus.removeAll(() => {
      void api.runtime.lastError;
      api.contextMenus.create({
        id: CONTEXT_MENU_ID,
        title: `Track on ${PRODUCT_NAME}`,
        contexts: ["selection"],
      });
    });
  }
}

/** Handles the "Track on Sorolens" right-click action on selected text. */
async function onContextMenuClick(info, tab) {
  const contractId = normalizeContractId((info && info.selectionText) || "");
  if (!contractId) return;
  const { client, settings } = await createClient();
  try {
    const outcome = await client.track(contractId, {
      network: settings.network || DEFAULT_NETWORK,
      watchlist: settings.trackWatchlist,
    });
    const status = await client.getTrackingStatus(contractId);
    const cache = await readStatusCache();
    await writeStatusCache(cache, [toCached(status)]);
    if (tab && typeof tab.id === "number") {
      await refreshBadge(tab.id, outcome.inWatchlist ? 1 : 0);
    }
  } catch {
    // Errors are surfaced through the popup's status list.
  }
}

/**
 * @param {import("../shared/messages.js").SorolensMessage} message
 * @param {{ tab?: { id?: number, url?: string } }} sender
 */
async function handleMessage(message, sender) {
  switch (message.type) {
    case MESSAGE.ping:
      return { ok: true, product: PRODUCT_NAME };

    case MESSAGE.getSettings:
      return { ok: true, settings: await getSettings() };

    case MESSAGE.saveSettings:
      return { ok: true, settings: await saveSettings(message.patch || {}) };

    case MESSAGE.testConnection: {
      const { client, settings } = await createClient();
      const origin = baseUrlToOriginPattern(settings.apiBaseUrl);
      if (!(await hasOrigin(origin))) {
        return {
          ok: false,
          error: `Host access to ${origin || settings.apiBaseUrl} is not granted. Open the options page and press Save to grant it.`,
        };
      }
      try {
        const started = Date.now();
        const health = await client.health();
        return {
          ok: true,
          latencyMs: Date.now() - started,
          apiBaseUrl: settings.apiBaseUrl,
          hasApiKey: client.hasApiKey,
          status: (health && health.status) || "ok",
        };
      } catch (error) {
        return { ok: false, error: describeError(error) };
      }
    }

    case MESSAGE.status: {
      const contractId = normalizeContractId(message.contractId || "");
      if (!contractId) return { ok: false, error: "Invalid contract ID" };
      const [status] = await resolveStatuses([contractId], {
        force: Boolean(message.force),
      });
      return { ok: true, status };
    }

    case MESSAGE.statusBatch: {
      const contractIds = (message.contractIds || [])
        .map((id) => normalizeContractId(id))
        .filter(Boolean)
        .slice(0, CONTENT.maxDecoratedIds);
      const statuses = await resolveStatuses(contractIds, {
        force: Boolean(message.force),
      });
      return { ok: true, statuses };
    }

    case MESSAGE.track: {
      const contractId = normalizeContractId(message.contractId || "");
      if (!contractId) return { ok: false, error: "Invalid contract ID" };
      const { client, settings } = await createClient();
      try {
        const outcome = await client.track(contractId, {
          network: message.network || settings.network,
          label: message.label,
          watchlist: settings.trackWatchlist,
        });
        const status = await client.getTrackingStatus(contractId);
        const cache = await readStatusCache();
        await writeStatusCache(cache, [toCached(status)]);
        return { ok: true, outcome, status };
      } catch (error) {
        return { ok: false, error: describeError(error) };
      }
    }

    case MESSAGE.untrack: {
      const contractId = normalizeContractId(message.contractId || "");
      if (!contractId) return { ok: false, error: "Invalid contract ID" };
      const { client } = await createClient();
      try {
        await client.untrack(contractId);
        const status = await client.getTrackingStatus(contractId);
        const cache = await readStatusCache();
        await writeStatusCache(cache, [toCached(status)]);
        return { ok: true, status };
      } catch (error) {
        return { ok: false, error: describeError(error) };
      }
    }

    case MESSAGE.scanResult: {
      const tabId = sender.tab && sender.tab.id;
      if (typeof tabId !== "number") return { ok: false };
      const contractIds = (message.contractIds || []).filter(Boolean);
      tabScans.set(tabId, {
        contractIds,
        url: message.url || (sender.tab && sender.tab.url) || "",
        at: Date.now(),
      });
      await refreshBadge(tabId, contractIds.length);
      return { ok: true };
    }

    case MESSAGE.getScan: {
      const scan = tabScans.get(message.tabId);
      return { ok: true, scan: scan || { contractIds: [], url: "" } };
    }

    default:
      return { ok: false, error: `Unsupported message: ${message.type}` };
  }
}

/** @param {string} origin */
async function hasOrigin(origin) {
  if (!origin) return false;
  const api = getBrowserApi();
  if (!api.permissions) return true;
  return new Promise((resolve) => {
    try {
      api.permissions.contains({ origins: [origin] }, (granted) => {
        void api.runtime.lastError;
        resolve(Boolean(granted));
      });
    } catch {
      resolve(false);
    }
  });
}

/** Wires the worker up. Kept in a function so tests can import this module. */
export function bootstrap() {
  const api = getBrowserApi();

  api.runtime.onInstalled.addListener((details) => {
    void onInstalled(details);
  });

  api.runtime.onMessage.addListener((message, sender, sendResponse) => {
    if (!isSorolensMessage(message)) return undefined;
    handleMessage(message, sender || {})
      .then(sendResponse)
      .catch((error) => sendResponse({ ok: false, error: describeError(error) }));
    // Keep the channel open for the async response (Chrome + Firefox).
    return true;
  });

  if (api.contextMenus) {
    api.contextMenus.onClicked.addListener((info, tab) => {
      if (info.menuItemId !== CONTEXT_MENU_ID) return;
      void onContextMenuClick(info, tab);
    });
  }

  if (api.tabs) {
    api.tabs.onUpdated.addListener((tabId, changeInfo) => {
      if (changeInfo.status === "loading") {
        tabScans.delete(tabId);
        void refreshBadge(tabId, 0);
      }
    });
    api.tabs.onRemoved.addListener((tabId) => {
      tabScans.delete(tabId);
    });
  }
}

// A browser context always exposes `chrome` (Chrome) or `browser` (Firefox).
// Node, which only imports this module for tests, exposes neither.
if (typeof globalThis.browser !== "undefined" || typeof globalThis.chrome !== "undefined") {
  bootstrap();
}

/** Exported for the unit tests in extension/tests. */
export const __test__ = {
  handleMessage,
  resolveStatuses,
  toCached,
  onContextMenuClick,
  refreshBadge,
  tabScans,
  CONTEXT_MENU_ID,
};
