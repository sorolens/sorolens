"use client";

import { type ReactNode } from "react";

export type SaveState = "idle" | "saved" | "error";

/**
 * One settings card. Each section is a self-contained widget with its own Save
 * button and its own status line, so a rejected value in one card never blocks
 * the others. State is owned by the page so every card reads from the same
 * source of truth.
 */
export function SettingsCard({
  title,
  description,
  children,
  status,
  onSave,
  testId,
  saveLabel = "Save",
}: {
  title: string;
  description: string;
  children: ReactNode;
  status: SaveState;
  onSave: () => boolean;
  testId: string;
  saveLabel?: string;
}) {
  return (
    <section
      data-testid={testId}
      aria-labelledby={`${testId}-heading`}
      className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-5"
    >
      <h2
        id={`${testId}-heading`}
        className="text-base font-semibold text-[var(--color-text-primary)]"
      >
        {title}
      </h2>
      <p className="mt-1 text-sm text-[var(--color-text-secondary)]">
        {description}
      </p>

      <div className="mt-4 space-y-3">{children}</div>

      <div className="mt-5 flex items-center gap-3">
        <button
          type="button"
          onClick={() => {
            onSave();
          }}
          className="rounded-md bg-[var(--color-accent)] px-3 py-1.5 text-sm font-medium text-white transition-opacity hover:opacity-90"
        >
          {saveLabel}
        </button>

        {status === "saved" && (
          <span
            data-testid={`${testId}-status`}
            role="status"
            aria-live="polite"
            className="text-xs text-[var(--color-text-secondary)]"
          >
            Saved
          </span>
        )}
        {status === "error" && (
          <span
            data-testid={`${testId}-status`}
            role="status"
            aria-live="polite"
            className="text-xs text-[var(--color-text-secondary)]"
          >
            Could not save — storage unavailable
          </span>
        )}
      </div>
    </section>
  );
}

/** Shared row wrapper so every toggle lines up the same way. */
export function SettingsRow({
  htmlFor,
  title,
  description,
  children,
}: {
  htmlFor: string;
  title: string;
  description: string;
  children: ReactNode;
}) {
  return (
    <div className="flex items-start justify-between gap-4">
      <div>
        <label
          htmlFor={htmlFor}
          className="text-sm font-medium text-[var(--color-text-primary)]"
        >
          {title}
        </label>
        <p className="text-xs text-[var(--color-text-secondary)]">
          {description}
        </p>
      </div>
      {children}
    </div>
  );
}
