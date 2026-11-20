/**
 * Keyboard shortcut plumbing for the dashboard shell (issue #831).
 *
 * This module holds the dependency-free pieces so they can be unit-tested
 * without a DOM: sequence matching, typing-target detection, and the persisted
 * "shortcuts disabled" preference.
 *
 * Shortcuts are sequences of one or two keys, e.g. "g c" or "/". A sequence is
 * armed by its first key and fires on the last key inside `SEQUENCE_TIMEOUT_MS`.
 */

export const SHORTCUTS_STORAGE_KEY = "sorolens_shortcuts_disabled";

/** Fired on `window` when the preference changes so live listeners re-read it. */
export const SHORTCUTS_CHANGED_EVENT = "sorolens:shortcuts-changed";

/** Fired on `window` by the `/` shortcut; the command palette listens for it. */
export const OPEN_SEARCH_EVENT = "sorolens:open-search";

/** How long the first key of a sequence stays armed. */
export const SEQUENCE_TIMEOUT_MS = 1000;

export type ShortcutBinding = {
  id: string;
  /** Space-separated key sequence, e.g. "g c" or "/". */
  sequence: string;
  label: string;
  run: () => void;
};

export type HelpEntry = {
  /** Rendering keys, e.g. ["g", "c"]. */
  keys: string[];
  label: string;
};

/** Help contents, in display order. Mirrors the runtime bindings. */
export const SHORTCUT_HELP: readonly HelpEntry[] = [
  { keys: ["g", "c"], label: "Go to Contracts" },
  { keys: ["g", "e"], label: "Go to Events" },
  { keys: ["g", "i"], label: "Go to Invocations" },
  { keys: ["/"], label: "Search contracts" },
  { keys: ["?"], label: "Show this help dialog" },
];

function resolveStorage(storage?: Storage): Storage | null {
  if (storage) return storage;
  if (typeof window === "undefined") return null;
  try {
    return window.localStorage;
  } catch {
    return null;
  }
}

/** Whether shortcuts are turned off. Defaults to enabled. */
export function isKeyboardShortcutsDisabled(storage?: Storage): boolean {
  const store = resolveStorage(storage);
  if (!store) return false;
  try {
    return store.getItem(SHORTCUTS_STORAGE_KEY) === "1";
  } catch {
    return false;
  }
}

/**
 * Persists the preference and notifies live listeners on `window`.
 * Returns false when storage is unavailable.
 */
export function setKeyboardShortcutsDisabled(
  disabled: boolean,
  storage?: Storage
): boolean {
  const store = resolveStorage(storage);
  let ok = false;
  if (store) {
    try {
      store.setItem(SHORTCUTS_STORAGE_KEY, disabled ? "1" : "0");
      ok = true;
    } catch {
      ok = false;
    }
  }

  if (typeof window !== "undefined") {
    window.dispatchEvent(new Event(SHORTCUTS_CHANGED_EVENT));
  }
  return ok;
}

/**
 * True while the user is typing somewhere shortcuts must not fire: form fields,
 * selects, and contenteditable regions (rich-text editors).
 */
export function isTypingTarget(target: unknown): boolean {
  if (!(target instanceof HTMLElement)) return false;
  const tag = target.tagName.toLowerCase();
  return (
    tag === "input" ||
    tag === "textarea" ||
    tag === "select" ||
    // jsdom leaves this undefined rather than false; coerce so the return
    // type is a real boolean.
    Boolean(target.isContentEditable)
  );
}

/** Splits "g c" into ["g", "c"]; collapses stray whitespace. */
export function parseSequence(sequence: string): string[] {
  return sequence.trim().toLowerCase().split(/\s+/).filter(Boolean);
}

export type Sequencer = {
  /** Feed one key (a `KeyboardEvent.key` value); returns the fired binding. */
  handle: (key: string, now?: number) => ShortcutBinding | null;
  /** Drops any armed prefix, e.g. when focus leaves the document. */
  reset: () => void;
};

type Armed = {
  binding: ShortcutBinding;
  keys: string[];
  /** Index of the key expected next. */
  next: number;
  expiresAt: number;
};

/**
 * Creates a stateful matcher for a fixed set of bindings.
 *
 * Several bindings may share a prefix (`g c`, `g e`, `g i`), so the sequencer
 * keeps every still-viable candidate armed instead of committing to the first
 * match. Modifier handling is deliberately *not* done here; the caller ignores
 * Cmd/Ctrl/Alt combinations so browser shortcuts keep working.
 */
export function createSequencer(
  bindings: ShortcutBinding[],
  timeoutMs: number = SEQUENCE_TIMEOUT_MS
): Sequencer {
  let armed: Armed[] = [];

  function reset() {
    armed = [];
  }

  function handle(key: string, now: number = Date.now()): ShortcutBinding | null {
    const pressed = key.toLowerCase();

    // Drop prefixes that timed out before considering this key.
    const live = armed.filter((candidate) => now <= candidate.expiresAt);

    // A sequence that ends on this key wins even if others continue.
    const completed = live.find(
      (candidate) =>
        candidate.keys[candidate.next] === pressed &&
        candidate.next + 1 === candidate.keys.length
    );
    if (completed) {
      reset();
      return completed.binding;
    }

    // Keep every candidate whose next key matches.
    const continuing: Armed[] = [];
    for (const candidate of live) {
      if (
        candidate.keys[candidate.next] === pressed &&
        candidate.next + 1 < candidate.keys.length
      ) {
        continuing.push({
          ...candidate,
          next: candidate.next + 1,
          expiresAt: now + timeoutMs,
        });
      }
    }

    if (continuing.length > 0) {
      armed = continuing;
      return null;
    }

    // Nothing continued, so this key starts a fresh sequence (this also clears
    // any stale prefix). Single-key shortcuts fire immediately.
    armed = [];
    for (const binding of bindings) {
      const keys = parseSequence(binding.sequence);
      if (keys.length === 0 || keys[0] !== pressed) continue;
      if (keys.length === 1) return binding;
      armed.push({ binding, keys, next: 1, expiresAt: now + timeoutMs });
    }

    return null;
  }

  return { handle, reset };
}
