// Stellar contract ID (StrKey) detection and validation.
//
// Used by the content script to find IDs on explorer pages, by the popup to
// validate manual input, and by the service worker before it calls the API.

import {
  BASE32_ALPHABET,
  CONTRACT_ID_LENGTH,
  CONTRACT_ID_PREFIX,
} from "./constants.js";

const CANDIDATE_RE = new RegExp(`${CONTRACT_ID_PREFIX}[A-Z2-7]{55}`, "g");

/** Zero-width characters that survive a copy/paste out of an explorer table. */
const INVISIBLE_RE = /[\u200B-\u200D\u2060\uFEFF]/g;

/**
 * Strict StrKey validation: 56 chars, `C` prefix, base32 alphabet only.
 * @param {unknown} value
 * @returns {boolean}
 */
export function isContractId(value) {
  if (typeof value !== "string" || value.length !== CONTRACT_ID_LENGTH) {
    return false;
  }
  if (value[0] !== CONTRACT_ID_PREFIX) {
    return false;
  }
  for (let i = 1; i < value.length; i += 1) {
    if (!BASE32_ALPHABET.includes(value[i])) {
      return false;
    }
  }
  return true;
}

/**
 * Normalizes user input (trim, strip zero-width chars, upper-case) and returns
 * the canonical ID, or `null` when the value is not a contract ID.
 * @param {string} raw
 * @returns {string | null}
 */
export function normalizeContractId(raw) {
  if (typeof raw !== "string") {
    return null;
  }
  const trimmed = raw.replace(INVISIBLE_RE, "").trim();
  if (isContractId(trimmed)) {
    return trimmed;
  }
  const upper = trimmed.toUpperCase();
  return isContractId(upper) ? upper : null;
}

/**
 * Finds every distinct contract ID in a blob of text.
 *
 * A candidate is rejected when the character immediately before or after it is
 * also base32, so IDs embedded in a longer token (a transaction hash, a base32
 * blob) do not produce false positives.
 *
 * @param {string} text
 * @returns {string[]} distinct IDs in order of first appearance
 */
export function findContractIds(text) {
  if (typeof text !== "string" || text.length < CONTRACT_ID_LENGTH) {
    return [];
  }
  const found = [];
  CANDIDATE_RE.lastIndex = 0;
  let match = CANDIDATE_RE.exec(text);
  while (match !== null) {
    const start = match.index;
    const end = start + CONTRACT_ID_LENGTH;
    const before = start > 0 ? text[start - 1] : "";
    const after = end < text.length ? text[end] : "";
    // `String.includes("")` is true, so empty boundaries must be handled first.
    const beforeIsBase32 = before !== "" && BASE32_ALPHABET.includes(before);
    const afterIsBase32 = after !== "" && BASE32_ALPHABET.includes(after);
    if (!beforeIsBase32 && !afterIsBase32 && !found.includes(match[0])) {
      found.push(match[0]);
    }
    CANDIDATE_RE.lastIndex = end;
    match = CANDIDATE_RE.exec(text);
  }
  return found;
}

/**
 * Pulls a contract ID out of an explorer URL such as
 * `https://stellar.expert/explorer/public/contract/C...` or `?contract=C...`.
 * @param {string} href
 * @returns {string | null}
 */
export function contractIdFromUrl(href) {
  if (typeof href !== "string" || href.length === 0) {
    return null;
  }
  const fromText = findContractIds(href);
  return fromText.length > 0 ? fromText[0] : null;
}

/** Shortens an ID for compact UI: `CABCDE…WXYZ`. */
export function shortenContractId(id, head = 6, tail = 4) {
  if (typeof id !== "string" || id.length <= head + tail + 1) {
    return typeof id === "string" ? id : "";
  }
  return `${id.slice(0, head)}…${id.slice(-tail)}`;
}
