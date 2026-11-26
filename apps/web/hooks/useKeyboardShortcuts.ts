"use client";

/**
 * Binds keyboard shortcuts to the document for the app shell (issue #831).
 *
 * The hook owns the DOM concerns: it ignores modifier combos so browser and
 * command-palette shortcuts still work, ignores events typed into form fields
 * or while an IME is composing, and stops firing while the user has disabled
 * shortcuts in Settings.
 */

import { useEffect, useState } from "react";

import {
  createSequencer,
  isKeyboardShortcutsDisabled,
  isTypingTarget,
  parseSequence,
  SHORTCUTS_CHANGED_EVENT,
  SEQUENCE_TIMEOUT_MS,
  type ShortcutBinding,
} from "@/lib/keyboard-shortcuts";

export function useKeyboardShortcuts(
  bindings: ShortcutBinding[],
  options: { enabled?: boolean } = {}
): void {
  const enabledByCaller = options.enabled ?? true;
  const [disabledBySettings, setDisabledBySettings] = useState(false);

  // Re-read the preference on mount, when Settings changes it, and when another
  // tab writes it. Not read during render so SSR and first client render match.
  useEffect(() => {
    const sync = () => setDisabledBySettings(isKeyboardShortcutsDisabled());
    sync();
    window.addEventListener(SHORTCUTS_CHANGED_EVENT, sync);
    window.addEventListener("storage", sync);
    return () => {
      window.removeEventListener(SHORTCUTS_CHANGED_EVENT, sync);
      window.removeEventListener("storage", sync);
    };
  }, []);

  useEffect(() => {
    if (!enabledByCaller || disabledBySettings) return;

    // Skip bindings that parse to nothing so a typo cannot render the app
    // permanently unresponsive to the first keystroke.
    const usable = bindings.filter(
      (binding) => parseSequence(binding.sequence).length > 0
    );
    if (usable.length === 0) return;

    const sequencer = createSequencer(usable, SEQUENCE_TIMEOUT_MS);

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.defaultPrevented) return;
      // Leave Cmd/Ctrl/Alt combos alone: ⌘K belongs to the command palette and
      // the rest belong to the browser.
      if (event.metaKey || event.ctrlKey || event.altKey) return;
      // Never steal keys from a form field or an active IME composition.
      if (event.isComposing || isTypingTarget(event.target)) return;

      const key = event.key === "/" && event.shiftKey ? "?" : event.key;
      const fired = sequencer.handle(key);
      if (fired) {
        event.preventDefault();
        fired.run();
      }
    };

    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [bindings, enabledByCaller, disabledBySettings]);
}
