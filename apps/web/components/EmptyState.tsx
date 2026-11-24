"use client";

import type { ReactNode } from "react";
import Link from "next/link";
import { ErrorState } from "@/components/ErrorState";

export type EmptyStateVariant = "first-run" | "no-results" | "error";

export interface EmptyStateAction {
  /** Visible label for the link or button. */
  label: string;
  /** Renders a link to this route instead of a button. */
  href?: string;
  /** Runs when the action is a button. */
  onClick?: () => void;
  /** Optional id so tests and shortcuts can target the action. */
  id?: string;
}

export interface EmptyStateProps {
  /**
   * `first-run` (nothing exists yet), `no-results` (data exists but the
   * filters match nothing) or `error` (the request failed).
   */
  variant?: EmptyStateVariant;
  /** Headline. Page-specific copy, taken from the i18n catalog. */
  title: ReactNode;
  /** Optional explanation, action hints and inline links. */
  description?: ReactNode;
  /** The single next action the empty state offers. */
  action?: EmptyStateAction;
  /** Error detail, only used by the error variant. */
  message?: string | null;
  /** Retry handler, only used by the error variant. */
  onRetry?: () => void;
  retrying?: boolean;
  retryLabel?: string;
  /** Smaller padding for empty states inside a card or section. */
  compact?: boolean;
  className?: string;
}

const actionClass =
  "inline-flex items-center rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm font-medium text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-accent)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent)]";

/**
 * Shared placeholder for a list with nothing to show. It distinguishes a
 * first run (nothing exists yet) from filters that match nothing, and its
 * `error` variant delegates to ErrorState so a failed request is never
 * mistaken for an empty list. Copy comes from the i18n catalog at the call
 * site, so every page explains itself and offers one next action.
 */
export function EmptyState({
  variant = "first-run",
  title,
  description,
  action,
  message,
  onRetry,
  retrying = false,
  retryLabel,
  compact = false,
  className = "",
}: EmptyStateProps) {
  if (variant === "error") {
    return (
      <ErrorState
        title={typeof title === "string" ? title : undefined}
        message={message}
        onRetry={onRetry}
        retrying={retrying}
        retryLabel={retryLabel}
        compact={compact}
        className={className}
      />
    );
  }

  return (
    <div
      role="status"
      data-empty-variant={variant}
      className={`rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] text-center ${
        variant === "no-results" ? "border-dashed" : ""
      } ${compact ? "p-6" : "px-8 py-16"} ${className}`.trim()}
    >
      <p className="text-lg font-medium text-[var(--color-text-primary)]">
        {title}
      </p>
      {description && (
        <div className="mt-2 text-sm text-[var(--color-text-secondary)]">
          {description}
        </div>
      )}
      {action && (
        <div className="mt-4">
          {action.href ? (
            <Link
              href={action.href}
              id={action.id}
              className={`${actionClass} no-underline`}
            >
              {action.label}
            </Link>
          ) : (
            <button
              type="button"
              id={action.id}
              onClick={action.onClick}
              className={actionClass}
            >
              {action.label}
            </button>
          )}
        </div>
      )}
    </div>
  );
}
