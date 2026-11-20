"use client";

import { useEffect, useState } from "react";

import { SettingsCard, SettingsRow, type SaveState } from "./SettingsCard";
import {
  isKeyboardShortcutsDisabled,
  setKeyboardShortcutsDisabled,
} from "@/lib/keyboard-shortcuts";

/**
 * Keyboard-shortcuts card (issue #831). Saving persists the preference to
 * `localStorage` and announces the change, so the shell stops handling
 * shortcuts immediately without a reload.
 */
export function KeyboardShortcutsSection() {
  // Start enabled to match the pre-hydration render, then read the stored
  // preference after mount so server and client markup agree.
  const [enabled, setEnabled] = useState(true);
  const [draft, setDraft] = useState(true);
  const [status, setStatus] = useState<SaveState>("idle");

  useEffect(() => {
    const stored = !isKeyboardShortcutsDisabled();
    setEnabled(stored);
    setDraft(stored);
  }, []);

  return (
    <SettingsCard
      testId="settings-shortcuts"
      title="Keyboard shortcuts"
      description="Navigate the dashboard without leaving the keyboard."
      status={status}
      onSave={() => {
        const ok = setKeyboardShortcutsDisabled(!draft);
        if (ok) setEnabled(draft);
        setStatus(ok ? "saved" : "error");
        return ok;
      }}
    >
      <SettingsRow
        htmlFor="settings-shortcuts-enabled"
        title="Enable keyboard shortcuts"
        description="g then c/e/i open Contracts, Events and Invocations; / searches; ? shows help."
      >
        <input
          id="settings-shortcuts-enabled"
          type="checkbox"
          checked={draft}
          onChange={(event) => {
            setDraft(event.target.checked);
            setStatus("idle");
          }}
          // Keep the checked state in sync with what is stored, so the row is
          // never ambiguous after a save.
          aria-describedby="settings-shortcuts-current"
          className="h-4 w-4 cursor-pointer accent-[var(--color-accent)]"
        />
      </SettingsRow>
      <p
        id="settings-shortcuts-current"
        className="text-xs text-[var(--color-text-secondary)]"
      >
        Shortcuts are currently {enabled ? "enabled" : "disabled"}.
      </p>
    </SettingsCard>
  );
}
