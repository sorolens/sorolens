"use client";

import { useEffect, useId, useRef } from "react";

import { SHORTCUT_HELP } from "@/lib/keyboard-shortcuts";

function KeyCap({ children }: { children: React.ReactNode }) {
  return (
    <kbd className="rounded border border-[var(--color-border)] bg-[var(--color-bg-page)] px-1.5 py-0.5 font-mono text-xs text-[var(--color-text-primary)]">
      {children}
    </kbd>
  );
}

/**
 * "?" help dialog listing the dashboard shortcuts (issue #831).
 *
 * Controlled by the parent so the `?` shortcut and the dialog stay in sync.
 * Escape closes it, the backdrop closes it, and focus moves into the dialog on
 * open and back to the previously focused element on close.
 */
export function HelpDialog({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const dialogRef = useRef<HTMLDivElement>(null);
  const restoreFocusRef = useRef<HTMLElement | null>(null);
  const titleId = useId();

  useEffect(() => {
    if (!open) return;

    restoreFocusRef.current =
      document.activeElement instanceof HTMLElement
        ? document.activeElement
        : null;
    dialogRef.current?.focus();

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        onClose();
      }
    };
    document.addEventListener("keydown", onKeyDown);

    return () => {
      document.removeEventListener("keydown", onKeyDown);
      restoreFocusRef.current?.focus();
    };
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-center bg-black/50 pt-[20vh] backdrop-blur-sm"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        className="w-full max-w-md overflow-hidden rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-card)] shadow-2xl outline-none"
      >
        <div className="flex items-center justify-between border-b border-[var(--color-border)] px-5 py-4">
          <h2
            id={titleId}
            className="text-base font-semibold text-[var(--color-text-primary)]"
          >
            Keyboard shortcuts
          </h2>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close keyboard shortcuts"
            className="text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
          >
            ✕
          </button>
        </div>

        <ul className="space-y-3 px-5 py-4">
          {SHORTCUT_HELP.map((entry) => (
            <li
              key={`${entry.keys.join("-")}-${entry.label}`}
              className="flex items-center justify-between gap-4 text-sm text-[var(--color-text-primary)]"
            >
              <span>{entry.label}</span>
              <span className="flex items-center gap-1">
                {entry.keys.map((key, index) => (
                  <span key={`${key}-${index}`} className="flex items-center gap-1">
                    {index > 0 && (
                      <span
                        aria-hidden
                        className="text-xs text-[var(--color-text-secondary)]"
                      >
                        then
                      </span>
                    )}
                    <KeyCap>{key}</KeyCap>
                  </span>
                ))}
              </span>
            </li>
          ))}
        </ul>

        <p className="border-t border-[var(--color-border)] px-5 py-3 text-xs text-[var(--color-text-secondary)]">
          Shortcuts are ignored while typing in a field. Turn them off in
          Settings.
        </p>
      </div>
    </div>
  );
}
