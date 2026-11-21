"use client";

import { useCallback, useMemo, useState } from "react";
import { useRouter } from "next/navigation";

import { HelpDialog } from "@/components/HelpDialog";
import { useKeyboardShortcuts } from "@/hooks/useKeyboardShortcuts";
import {
  OPEN_SEARCH_EVENT,
  type ShortcutBinding,
} from "@/lib/keyboard-shortcuts";

/**
 * Registers the dashboard's global shortcuts and renders the `?` help dialog.
 *
 * Mounted once from the app-shell layout so every dashboard route gets the same
 * shortcuts. Navigation goes through the Next router, so these keep client-side
 * transitions instead of full reloads.
 */
export function AppShortcuts() {
  const router = useRouter();
  const [helpOpen, setHelpOpen] = useState(false);
  const closeHelp = useCallback(() => setHelpOpen(false), []);

  const bindings = useMemo<ShortcutBinding[]>(
    () => [
      {
        id: "goto-contracts",
        sequence: "g c",
        label: "Go to Contracts",
        run: () => router.push("/contracts"),
      },
      {
        id: "goto-events",
        sequence: "g e",
        label: "Go to Events",
        run: () => router.push("/events"),
      },
      {
        id: "goto-invocations",
        sequence: "g i",
        label: "Go to Invocations",
        run: () => router.push("/invocations"),
      },
      {
        id: "open-search",
        sequence: "/",
        label: "Search contracts",
        // The command palette owns its open state, so ask it to open rather
        // than duplicating that state here.
        run: () => window.dispatchEvent(new Event(OPEN_SEARCH_EVENT)),
      },
      {
        id: "open-help",
        sequence: "?",
        label: "Show keyboard shortcuts",
        run: () => setHelpOpen(true),
      },
    ],
    [router]
  );

  // While the dialog is up its own Escape handling owns the keyboard.
  useKeyboardShortcuts(bindings, { enabled: !helpOpen });

  return <HelpDialog open={helpOpen} onClose={closeHelp} />;
}
