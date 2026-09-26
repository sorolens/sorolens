// Options page: API key, identity, API base URL and inline-button preferences.

import {
  queryTabs,
  requestHostPermission,
  sendRuntimeMessage,
  sendTabMessage,
} from "../shared/browser.js";
import { NETWORKS } from "../shared/constants.js";
import { MESSAGE } from "../shared/messages.js";
import { baseUrlToOriginPattern } from "../shared/storage.js";

const el = {
  form: document.getElementById("settings-form"),
  apiKey: document.getElementById("api-key"),
  revealKey: document.getElementById("reveal-key"),
  userId: document.getElementById("user-id"),
  apiBaseUrl: document.getElementById("api-base-url"),
  network: document.getElementById("network"),
  trackWatchlist: document.getElementById("track-watchlist"),
  showInlineButtons: document.getElementById("show-inline-buttons"),
  saveButton: document.getElementById("save-button"),
  testButton: document.getElementById("test-button"),
  clearKeyButton: document.getElementById("clear-key-button"),
  notice: document.getElementById("notice"),
};

/** @param {string} message @param {"ok" | "error" | "info"} tone */
function setNotice(message, tone = "info") {
  if (!message) {
    el.notice.hidden = true;
    el.notice.textContent = "";
    return;
  }
  el.notice.hidden = false;
  el.notice.textContent = message;
  el.notice.dataset.tone = tone;
}

function populateNetworks() {
  el.network.textContent = "";
  for (const network of NETWORKS) {
    const option = document.createElement("option");
    option.value = network;
    option.textContent = network;
    el.network.append(option);
  }
}

/** @param {import("../shared/storage.js").SorolensSettings} value */
function fillForm(value) {
  el.apiKey.value = value.apiKey;
  el.revealKey.checked = false;
  el.apiKey.type = "password";
  el.userId.value = value.userId;
  el.apiBaseUrl.value = value.apiBaseUrl;
  el.network.value = NETWORKS.includes(value.network)
    ? value.network
    : NETWORKS[0];
  el.trackWatchlist.checked = value.trackWatchlist;
  el.showInlineButtons.checked = value.showInlineButtons;
}

async function load() {
  const response = await sendRuntimeMessage({ type: MESSAGE.getSettings });
  if (!response || !response.ok) {
    setNotice("Could not read the stored settings.", "error");
    return;
  }
  fillForm(response.settings);
}

/** Asks every supported tab to re-run its scan after a settings change. */
async function rescanSupportedTabs() {
  try {
    const tabs = await queryTabs({});
    await Promise.all(
      tabs
        .filter((tab) => typeof tab.id === "number")
        .map((tab) =>
          sendTabMessage(tab.id, { type: MESSAGE.rescan }).catch(() => undefined),
        ),
    );
  } catch {
    // Tabs that are not running the content script simply do not answer.
  }
}

async function onSave(event) {
  event.preventDefault();
  el.saveButton.disabled = true;
  setNotice("");

  const apiBaseUrl = el.apiBaseUrl.value.trim();
  const origin = baseUrlToOriginPattern(apiBaseUrl);
  if (!origin) {
    el.saveButton.disabled = false;
    setNotice("The API base URL must be a valid http(s) URL.", "error");
    return;
  }

  // Ask for host access while the click gesture is still valid.
  const granted = await requestHostPermission(origin);
  if (!granted) {
    el.saveButton.disabled = false;
    setNotice(`Browser access to ${origin} was not granted.`, "error");
    return;
  }

  const response = await sendRuntimeMessage({
    type: MESSAGE.saveSettings,
    patch: {
      apiKey: el.apiKey.value,
      userId: el.userId.value,
      apiBaseUrl,
      network: el.network.value,
      trackWatchlist: el.trackWatchlist.checked,
      showInlineButtons: el.showInlineButtons.checked,
    },
  });
  el.saveButton.disabled = false;
  if (!response || !response.ok) {
    setNotice("Saving the settings failed.", "error");
    return;
  }
  fillForm(response.settings);
  setNotice("Settings saved.", "ok");
  void rescanSupportedTabs();
}

async function onTest() {
  el.testButton.disabled = true;
  setNotice("Contacting the Sorolens API…");
  const response = await sendRuntimeMessage({ type: MESSAGE.testConnection });
  el.testButton.disabled = false;
  if (!response || !response.ok) {
    setNotice((response && response.error) || "Connection test failed.", "error");
    return;
  }
  const keyState = response.hasApiKey ? "with an API key" : "without an API key";
  setNotice(
    `Connected to ${response.apiBaseUrl} in ${response.latencyMs} ms ${keyState}.`,
    "ok",
  );
}

async function onClearKey() {
  el.clearKeyButton.disabled = true;
  const response = await sendRuntimeMessage({
    type: MESSAGE.saveSettings,
    patch: { apiKey: "" },
  });
  el.clearKeyButton.disabled = false;
  if (!response || !response.ok) {
    setNotice("Could not clear the API key.", "error");
    return;
  }
  fillForm(response.settings);
  setNotice("API key removed from this device.", "ok");
}

el.revealKey.addEventListener("change", () => {
  el.apiKey.type = el.revealKey.checked ? "text" : "password";
});
el.form.addEventListener("submit", (event) => void onSave(event));
el.testButton.addEventListener("click", () => void onTest());
el.clearKeyButton.addEventListener("click", () => void onClearKey());

populateNetworks();
void load();
