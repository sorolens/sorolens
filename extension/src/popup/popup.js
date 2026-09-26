// Popup: shows the tracking status of the contracts on the active tab and
// offers a one-click add (plus a manual paste box for anywhere else).

import { normalizeContractId, shortenContractId } from "../shared/contract-id.js";
import {
  getActiveTab,
  openOptionsPage,
  sendRuntimeMessage,
  sendTabMessage,
} from "../shared/browser.js";
import { MESSAGE, maskApiKey } from "../shared/messages.js";

const el = {
  connectionDot: document.getElementById("connection-dot"),
  detectedPanel: document.getElementById("detected-panel"),
  detectedList: document.getElementById("detected-list"),
  emptyState: document.getElementById("empty-state"),
  manualForm: document.getElementById("manual-form"),
  manualInput: document.getElementById("manual-input"),
  manualHint: document.getElementById("manual-hint"),
  notice: document.getElementById("notice"),
  meta: document.getElementById("meta"),
  refreshButton: document.getElementById("refresh-button"),
  optionsButton: document.getElementById("options-button"),
};

/** @type {import("../shared/storage.js").SorolensSettings | null} */
let settings = null;
/** contractId -> TrackingStatus */
let rows = new Map();
let busy = false;

/** Shows a message under the panels; `tone="info"` for non-errors. */
function setNotice(message, tone = "error") {
  if (!message) {
    el.notice.hidden = true;
    el.notice.textContent = "";
    return;
  }
  el.notice.hidden = false;
  el.notice.textContent = message;
  el.notice.dataset.tone = tone;
}

function setConnection(state) {
  el.connectionDot.dataset.state = state;
  if (state === "ok") {
    el.connectionDot.title = "Connected to the Sorolens API";
  } else if (state === "error") {
    el.connectionDot.title = "Sorolens API unreachable";
  } else {
    el.connectionDot.title = "Checking API connection…";
  }
}

/** Renders the meta line: API host, network and whether a key is stored. */
function renderMeta() {
  if (!settings) {
    el.meta.textContent = "";
    return;
  }
  const host = settings.apiBaseUrl.replace(/^https?:\/\//, "");
  const key = settings.apiKey
    ? `key ${maskApiKey(settings.apiKey)}`
    : "no API key";
  el.meta.textContent = `${host} · ${settings.network} · ${key}`;
}

/** @param {import("../shared/api.js").TrackingStatus} status */
function rowBadge(status) {
  const badge = document.createElement("span");
  badge.className = "badge";
  if (status.error) {
    badge.dataset.state = "error";
    badge.textContent = "Error";
    badge.title = status.error;
  } else if (status.inWatchlist) {
    badge.dataset.state = "watched";
    badge.textContent = "Tracked";
  } else if (status.tracked) {
    badge.dataset.state = "indexed";
    badge.textContent = "Indexed";
    badge.title = "Indexed on Sorolens, not on your watchlist";
  } else {
    badge.textContent = "New";
  }
  return badge;
}

/**
 * @param {string} contractId
 * @param {import("../shared/api.js").TrackingStatus} status
 */
function rowElement(contractId, status) {
  const row = document.createElement("li");
  row.className = "row";

  const code = document.createElement("code");
  code.textContent = shortenContractId(contractId);
  code.title = contractId;

  row.append(code, rowBadge(status));

  const action = document.createElement("button");
  action.type = "button";
  action.className = "button link";
  const watched = Boolean(status && status.inWatchlist);
  action.textContent = watched ? "Remove" : "Track";
  action.disabled = busy;
  action.addEventListener("click", () => {
    void (watched ? untrack(contractId) : track(contractId));
  });
  row.append(action);
  return row;
}

function render() {
  el.detectedList.textContent = "";
  const entries = [...rows.entries()];
  el.detectedPanel.hidden = entries.length === 0;
  el.emptyState.hidden = entries.length > 0;
  for (const [contractId, status] of entries) {
    el.detectedList.append(rowElement(contractId, status));
  }
  renderMeta();
}

/** Collects the contract IDs of the active tab, with a service-worker fallback. */
async function collectDetected() {
  const tab = await getActiveTab();
  if (!tab || typeof tab.id !== "number") {
    setNotice("No active tab to inspect.", "info");
    return [];
  }
  try {
    const response = await sendTabMessage(tab.id, { type: MESSAGE.getDetected });
    if (response && Array.isArray(response.contractIds)) {
      return response.contractIds;
    }
  } catch {
    // Not a supported page, or the content script is still starting up.
  }
  const fallback = await sendRuntimeMessage({
    type: MESSAGE.getScan,
    tabId: tab.id,
  });
  if (fallback && fallback.ok && fallback.scan) {
    return fallback.scan.contractIds || [];
  }
  return [];
}

/** @param {string[]} contractIds */
async function loadStatuses(contractIds, force) {
  if (contractIds.length === 0) return;
  const response = await sendRuntimeMessage({
    type: MESSAGE.statusBatch,
    contractIds,
    force,
  });
  if (!response || !response.ok) {
    setConnection("error");
    setNotice(
      (response && response.error) || "Could not reach the Sorolens API",
    );
    return;
  }
  rows = new Map(response.statuses.map((status) => [status.contractId, status]));
  const firstError = response.statuses.find((status) => status.error);
  if (firstError) {
    setConnection("error");
    setNotice(`${shortenContractId(firstError.contractId)}: ${firstError.error}`);
  } else {
    setConnection("ok");
    setNotice("");
  }
}

/** @param {string} contractId */
async function track(contractId) {
  busy = true;
  render();
  const response = await sendRuntimeMessage({ type: MESSAGE.track, contractId });
  busy = false;
  if (!response || !response.ok) {
    setNotice((response && response.error) || "Tracking failed");
    setConnection("error");
    render();
    return;
  }
  const warnings = (response.outcome && response.outcome.warnings) || [];
  setNotice(warnings.join(" "));
  rows.set(contractId, response.status);
  render();
}

/** @param {string} contractId */
async function untrack(contractId) {
  busy = true;
  render();
  const response = await sendRuntimeMessage({
    type: MESSAGE.untrack,
    contractId,
  });
  busy = false;
  if (!response || !response.ok) {
    setNotice((response && response.error) || "Removing failed");
    render();
    return;
  }
  setNotice("");
  rows.set(contractId, response.status);
  render();
}

async function loadSettings() {
  const response = await sendRuntimeMessage({ type: MESSAGE.getSettings });
  if (response && response.ok) {
    settings = response.settings;
  }
  renderMeta();
}

async function refresh({ force = false } = {}) {
  await loadSettings();
  const detected = await collectDetected();
  const ids = new Set(detected);
  for (const contractId of rows.keys()) ids.add(contractId);
  if (ids.size === 0) {
    render();
    setConnection(settings && settings.apiKey ? "unknown" : "error");
    if (!settings || !settings.apiKey) {
      setNotice(
        "Add your Sorolens API key in Settings to register untracked contracts.",
        "info",
      );
    }
    return;
  }
  await loadStatuses([...ids], force);
  render();
}

function onSubmit(event) {
  event.preventDefault();
  const contractId = normalizeContractId(el.manualInput.value);
  if (!contractId) {
    el.manualHint.textContent =
      "That does not look like a Soroban contract ID (C + 55 base32 characters).";
    el.manualHint.dataset.tone = "error";
    return;
  }
  el.manualHint.textContent = `Tracking ${shortenContractId(contractId)}…`;
  el.manualHint.dataset.tone = "info";
  el.manualInput.value = "";
  if (!rows.has(contractId)) {
    rows.set(contractId, { contractId, tracked: false, inWatchlist: false });
  }
  void track(contractId).then(() => {
    el.manualHint.textContent =
      "Paste a 56-character Soroban contract ID (starts with C).";
    el.manualHint.dataset.tone = "";
  });
}

el.manualForm.addEventListener("submit", onSubmit);
el.refreshButton.addEventListener("click", () => {
  void refresh({ force: true });
});
el.optionsButton.addEventListener("click", () => {
  void openOptionsPage();
});

void refresh();
