"use client";

import { useEffect, useRef } from "react";

export interface ConfirmDialogProps {
  isOpen: boolean;
  title: string;
  description: string;
  confirmText?: string;
  cancelText?: string;
  isDestructive?: boolean;
  isLoading?: boolean;
  actionName?: string;
  onConfirm: () => void | Promise<void>;
  onClose: () => void;
}

/**
 * Accessible confirmation dialog for destructive actions (issue #763).
 *
 * Implements WAI-ARIA Dialog (Modal) pattern:
 * - role="dialog", aria-modal="true"
 * - aria-labelledby and aria-describedby
 * - Focus trap (Tab / Shift+Tab cycling)
 * - Initial focus on cancel button to prevent accidental submission
 * - Dismissable via Escape key and backdrop click
 */
export function ConfirmDialog({
  isOpen,
  title,
  description,
  confirmText = "Confirm",
  cancelText = "Cancel",
  isDestructive = true,
  isLoading = false,
  actionName,
  onConfirm,
  onClose,
}: ConfirmDialogProps) {
  const dialogRef = useRef<HTMLDivElement>(null);
  const cancelBtnRef = useRef<HTMLButtonElement>(null);

  // Focus trap and Escape key listener
  useEffect(() => {
    if (!isOpen) return;

    // Focus cancel button on open
    const timer = setTimeout(() => {
      cancelBtnRef.current?.focus();
    }, 10);

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        if (!isLoading) {
          e.preventDefault();
          onClose();
        }
        return;
      }

      if (e.key === "Tab") {
        if (!dialogRef.current) return;
        const focusable = dialogRef.current.querySelectorAll<HTMLElement>(
          'button:not([disabled]), [tabindex]:not([tabindex="-1"])'
        );
        if (focusable.length === 0) return;

        const firstElement = focusable[0];
        const lastElement = focusable[focusable.length - 1];

        if (e.shiftKey) {
          if (document.activeElement === firstElement) {
            e.preventDefault();
            lastElement.focus();
          }
        } else {
          if (document.activeElement === lastElement) {
            e.preventDefault();
            firstElement.focus();
          }
        }
      }
    };

    document.addEventListener("keydown", handleKeyDown);
    return () => {
      clearTimeout(timer);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [isOpen, isLoading, onClose]);

  if (!isOpen) return null;

  return (
    <div
      role="presentation"
      data-testid="confirm-dialog-backdrop"
      onClick={(e) => {
        if (e.target === e.currentTarget && !isLoading) {
          onClose();
        }
      }}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm"
    >
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="confirm-dialog-title"
        aria-describedby="confirm-dialog-desc"
        data-testid="confirm-dialog"
        className="w-full max-w-md rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-card)] p-6 shadow-2xl"
      >
        <h2
          id="confirm-dialog-title"
          className="text-lg font-semibold text-[var(--color-text-primary)]"
        >
          {title}
        </h2>
        <p
          id="confirm-dialog-desc"
          className="mt-2 text-sm text-[var(--color-text-secondary)] leading-relaxed"
        >
          {description}
        </p>

        <div className="mt-6 flex justify-end gap-3">
          <button
            ref={cancelBtnRef}
            type="button"
            data-testid="confirm-dialog-cancel"
            onClick={onClose}
            disabled={isLoading}
            className="rounded-lg border border-[var(--color-border)] px-4 py-2 text-sm font-medium text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)] hover:border-[var(--color-accent)] disabled:opacity-50"
          >
            {cancelText}
          </button>
          <button
            type="button"
            data-testid="confirm-dialog-confirm"
            aria-label={actionName ?? confirmText}
            onClick={() => void onConfirm()}
            disabled={isLoading}
            className={`rounded-lg px-4 py-2 text-sm font-medium text-[var(--color-text-primary)] transition-opacity disabled:opacity-50 ${
              isDestructive
                ? "bg-[var(--color-danger)] hover:bg-[var(--color-danger-hover)]"
                : "bg-[var(--color-accent)] hover:opacity-90"
            }`}
          >
            {isLoading ? "Processing…" : confirmText}
          </button>
        </div>
      </div>
    </div>
  );
}
