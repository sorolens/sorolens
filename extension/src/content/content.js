// Page-side injection: find contract IDs on Stellar explorers and render a
// "Track on Sorolens" button next to them.
//
// Design notes:
// * Buttons are inserted *after* the matched text node or link, never by
//   rewriting the page's text nodes. Explorer UIs are React apps, and mutating
//   their text nodes would fight hydration and re-renders.
// * Everything the script inserts carries `data-sorolens-ui`, so the
//   MutationObserver can tell its own mutations apart and stop rescanning.
// * All network work happens in the service worker (see ../background).

import {
  CONTRACT_ID_LENGTH,
  CONTENT,
  PRODUCT_NAME,
} from "../shared/constants.js";
import {
  contractIdFromUrl,
  findContractIds,
  shortenContractId,
} from "../shared/contract-id.js";
import { MESSAGE } from "../shared/messages.js";
import { getBrowserApi } from "../shared/browser.js";

const UI_ATTRIBUTE = "data-sorolens-ui";
const CONTRACT_ATTRIBUTE = "data-sorolens-contract";
const STATE_ATTRIBUTE = "data-sorolens-state";
const BUTTON_SELECTOR = `[${UI_ATTRIBUTE}="button"]`;

/** Elements whose text must never be decorated. */
const SKIP_SELECTOR = [
  "script",
  "style",
  "noscript",
  "textarea",
  "input",
  "select",
  "option",
  "svg",
  "[contenteditable='true']",
  `[${UI_ATTRIBUTE}]`,
].join(",");

/** Upper bound on text nodes visited per scan, to keep big pages responsive. */
const MAX_TEXT_NODES = 20000;

/** @type {{ showInlineButtons: boolean }} */
let settings = { showInlineButtons: true };

/** contractId -> inserted button count, so one ID cannot carpet a page. */
const buttonCounts = new Map();

/** container element -> set of contract IDs already decorated inside it. */
const decoratedContainers = new WeakMap();

let lastScanIds = [];
let scanTimer = null;

const api = getBrowserApi();

/** True when the element may have UI inserted around it. */
function isScannable(element) {
  if (!element || !element.isConnected) return false;
  return !element.closest(SKIP_SELECTOR);
}

/** Promise wrapper for `runtime.sendMessage` that never rejects. */
function sendMessage(message) {
  return new Promise((resolve) => {
    let settled = false;
    const done = (value) => {
      if (!settled) {
        settled = true;
        resolve(value);
      }
    };
    try {
      const maybe = api.runtime.sendMessage(message, (response) => {
        done(api.runtime.lastError ? null : response);
      });
      if (maybe && typeof maybe.then === "function") {
        maybe.then(done, () => done(null));
      }
    } catch {
      done(null);
    }
  });
}

/** Creates the inline button for a contract ID. */
function createButton(contractId) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = "sorolens-track-btn";
  button.setAttribute(UI_ATTRIBUTE, "button");
  button.setAttribute(CONTRACT_ATTRIBUTE, contractId);
  button.setAttribute(STATE_ATTRIBUTE, "idle");
  button.title = `Track ${shortenContractId(contractId)} on ${PRODUCT_NAME}`;

  const icon = document.createElement("span");
  icon.className = "sorolens-track-btn__icon";
  icon.setAttribute("aria-hidden", "true");
  icon.textContent = "◎";

  const label = document.createElement("span");
  label.className = "sorolens-track-btn__label";
  label.textContent = "Track";

  button.append(icon, label);
  return button;
}

/**
 * Applies a visual state, keeping the label and tooltip in sync.
 * @param {HTMLButtonElement} button
 * @param {"idle" | "pending" | "watched" | "indexed" | "error"} state
 * @param {string} [detail] tooltip override
 */
function setButtonState(button, state, detail) {
  button.setAttribute(STATE_ATTRIBUTE, state);
  const label = button.querySelector(".sorolens-track-btn__label");
  const contractId = button.getAttribute(CONTRACT_ATTRIBUTE) || "";
  const short = shortenContractId(contractId);

  switch (state) {
    case "pending":
      button.disabled = true;
      if (label) label.textContent = "Tracking…";
      button.title = `Contacting ${PRODUCT_NAME}…`;
      break;
    case "watched":
      button.disabled = false;
      if (label) label.textContent = "Tracked";
      button.title = detail || `Tracked on ${PRODUCT_NAME} — click to remove`;
      break;
    case "indexed":
      button.disabled = false;
      if (label) label.textContent = "Track";
      button.title =
        detail || `${short} is indexed on ${PRODUCT_NAME} — add it to your watchlist`;
      break;
    case "error":
      button.disabled = false;
      if (label) label.textContent = "Retry";
      button.title = detail || "Tracking failed — click to retry";
      break;
    default:
      button.disabled = false;
      if (label) label.textContent = "Track";
      button.title = detail || `Track ${short} on ${PRODUCT_NAME}`;
      break;
  }
}

/**
 * Inserts a button next to `reference` (a text node or an element).
 * Text nodes cannot use `insertAdjacentElement`, so they take the
 * `insertBefore` path against their parent.
 */
function insertButtonAfter(reference, contractId) {
  const container = reference.parentElement;
  if (!container || !isScannable(container)) return false;

  const decorated = decoratedContainers.get(container) || new Set();
  if (decorated.has(contractId)) return false;
  const count = buttonCounts.get(contractId) || 0;
  if (count >= CONTENT.maxButtonsPerId) return false;

  const button = createButton(contractId);
  if (reference.nodeType === Node.TEXT_NODE) {
    container.insertBefore(button, reference.nextSibling);
  } else {
    reference.insertAdjacentElement("afterend", button);
  }
  decorated.add(contractId);
  decoratedContainers.set(container, decorated);
  buttonCounts.set(contractId, count + 1);
  return true;
}

/**
 * Walks the page for contract IDs.
 * @returns {{ ids: string[], anchors: Map<string, Node> }}
 */
function scanPage() {
  const ids = new Set();
  const anchors = new Map();
  const root = document.body || document.documentElement;
  if (!root) return { ids: [], anchors };

  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  let node = walker.nextNode();
  let visited = 0;

  while (node && visited < MAX_TEXT_NODES && ids.size <= CONTENT.maxDecoratedIds) {
    visited += 1;
    const parent = node.parentElement;
    const value = node.nodeValue || "";
    if (parent && value.length >= CONTRACT_ID_LENGTH && isScannable(parent)) {
      for (const id of findContractIds(value)) {
        if (ids.size > CONTENT.maxDecoratedIds) break;
        ids.add(id);
        if (anchors.has(id)) continue;
        // Inside a link, decorate after the link so the injected button is not
        // a nested interactive element.
        const link = parent.closest("a");
        anchors.set(id, link || node);
      }
    }
    node = walker.nextNode();
  }

  for (const link of document.querySelectorAll("a[href]")) {
    if (ids.size > CONTENT.maxDecoratedIds) break;
    const href = link.getAttribute("href") || "";
    if (!href.includes("contract") && !href.includes("C")) continue;
    const id = contractIdFromUrl(href);
    if (!id) continue;
    ids.add(id);
    if (!anchors.has(id)) anchors.set(id, link);
  }

  const found = [...ids].slice(0, CONTENT.maxDecoratedIds);
  return { ids: found, anchors };
}

/** Inserts one button per distinct ID found on the page. */
function decorate(anchors, ids) {
  for (const id of ids) {
    const reference = anchors.get(id);
    if (!reference) continue;
    try {
      insertButtonAfter(reference, id);
    } catch {
      // Detached nodes during SPA navigation are expected; skip quietly.
    }
  }
}

/** Fetches statuses for the IDs on the page and reflects them on the buttons. */
async function refreshButtonStates(ids) {
  if (ids.length === 0) return;
  const response = await sendMessage({
    type: MESSAGE.statusBatch,
    contractIds: ids,
  });
  if (!response || !response.ok || !Array.isArray(response.statuses)) return;
  for (const status of response.statuses) {
    const buttons = document.querySelectorAll(
      `${BUTTON_SELECTOR}[${CONTRACT_ATTRIBUTE}="${status.contractId}"]`,
    );
    for (const button of buttons) {
      if (button.getAttribute(STATE_ATTRIBUTE) === "pending") continue;
      if (status.inWatchlist) {
        setButtonState(button, "watched");
      } else if (status.tracked) {
        setButtonState(button, "indexed", status.error);
      } else {
        setButtonState(button, "idle", status.error);
      }
    }
  }
}

/** Click handling for every injected button, delegated at the document level. */
async function onClick(event) {
  const target = event.target;
  const button =
    target instanceof Element ? target.closest(BUTTON_SELECTOR) : null;
  if (!button) return;
  event.preventDefault();
  event.stopPropagation();

  const contractId = button.getAttribute(CONTRACT_ATTRIBUTE);
  if (!contractId) return;

  if (button.getAttribute(STATE_ATTRIBUTE) === "watched") {
    setButtonState(button, "pending");
    const response = await sendMessage({ type: MESSAGE.untrack, contractId });
    if (response && response.ok) {
      setButtonState(button, "idle");
    } else {
      setButtonState(button, "error", response && response.error);
    }
    return;
  }

  setButtonState(button, "pending");
  const response = await sendMessage({ type: MESSAGE.track, contractId });
  if (!response || !response.ok) {
    setButtonState(button, "error", response && response.error);
    return;
  }
  const warnings = (response.outcome && response.outcome.warnings) || [];
  const inWatchlist = Boolean(response.status && response.status.inWatchlist);
  if (inWatchlist) {
    setButtonState(button, "watched", warnings.join(" ") || undefined);
  } else {
    setButtonState(
      button,
      "indexed",
      warnings.join(" ") || "Registered for indexing — watchlist add failed",
    );
  }
}

/** Reports the IDs on this page to the service worker (badge + popup). */
function report(ids) {
  lastScanIds = ids;
  void sendMessage({
    type: MESSAGE.scanResult,
    contractIds: ids,
    url: location.href,
  });
}

/** Runs one scan cycle: collect, decorate, report, refresh states. */
function runScan() {
  const { ids, anchors } = scanPage();
  if (settings.showInlineButtons) decorate(anchors, ids);
  report(ids);
  void refreshButtonStates(ids);
}

/** Debounced rescan used by the MutationObserver. */
function scheduleRescan() {
  if (scanTimer !== null) return;
  scanTimer = window.setTimeout(() => {
    scanTimer = null;
    runScan();
  }, CONTENT.rescanDebounceMs);
}

/** True when a mutation batch only contains our own UI, or nothing. */
function isOwnMutation(record) {
  const nodes = [
    ...Array.from(record.addedNodes || []),
    ...Array.from(record.removedNodes || []),
  ];
  if (nodes.length === 0) return true;
  return nodes.every((node) => {
    if (node.nodeType !== Node.ELEMENT_NODE) return false;
    return (
      node.hasAttribute(UI_ATTRIBUTE) ||
      Boolean(node.closest(`[${UI_ATTRIBUTE}]`))
    );
  });
}

/** Refreshes settings from the service worker. */
async function loadSettings() {
  const response = await sendMessage({ type: MESSAGE.getSettings });
  if (response && response.ok && response.settings) {
    settings = {
      showInlineButtons: response.settings.showInlineButtons !== false,
    };
  }
}

/** Removes every injected button; used when inline buttons are turned off. */
function removeButtons() {
  for (const button of document.querySelectorAll(BUTTON_SELECTOR)) {
    button.remove();
  }
  buttonCounts.clear();
}

/** Entry point, called by the classic loader (src/content/loader.js). */
export async function start() {
  await loadSettings();

  document.addEventListener("click", (event) => void onClick(event), true);

  api.runtime.onMessage.addListener((message, _sender, sendResponse) => {
    if (!message || typeof message.type !== "string") return undefined;
    if (message.type === MESSAGE.getDetected) {
      sendResponse({ contractIds: lastScanIds, url: location.href });
      return undefined;
    }
    if (message.type === MESSAGE.rescan) {
      void loadSettings().then(() => {
        if (!settings.showInlineButtons) removeButtons();
        runScan();
      });
      sendResponse({ ok: true });
      return undefined;
    }
    return undefined;
  });

  runScan();

  if (typeof MutationObserver === "function") {
    const observer = new MutationObserver((records) => {
      if (records.every(isOwnMutation)) return;
      scheduleRescan();
    });
    observer.observe(document.documentElement, {
      childList: true,
      subtree: true,
    });
  }
}

/** Exposed for the unit tests. */
export const __test__ = {
  scanPage,
  decorate,
  insertButtonAfter,
  setButtonState,
  createButton,
  isOwnMutation,
  UI_ATTRIBUTE,
  CONTRACT_ATTRIBUTE,
  STATE_ATTRIBUTE,
};
