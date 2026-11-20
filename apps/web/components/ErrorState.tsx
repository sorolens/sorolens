"use client";

interface ErrorStateProps {
  /** Short headline, e.g. "Couldn't load contracts." */
  title?: string;
  /** Optional detail (typically the error message) shown under the title. */
  message?: string | null;
  /**
   * Called when the user presses the retry button. Omit to render the error
   * without a retry action.
   */
  onRetry?: () => void;
  /** Disables the retry button and shows progress while a retry is running. */
  retrying?: boolean;
  /** Retry button label. */
  retryLabel?: string;
  /** Smaller padding for errors that sit inside a section or card. */
  compact?: boolean;
  className?: string;
}

/**
 * Shared failure banner for data fetches. Pages render it instead of an empty
 * state when a request fails, so an API outage is never mistaken for "no
 * data", and offer a retry that re-runs the failed request.
 */
export function ErrorState({
  title = "Couldn't load this data.",
  message,
  onRetry,
  retrying = false,
  retryLabel = "Try again",
  compact = false,
  className = "",
}: ErrorStateProps) {
  return (
    <div
      role="alert"
      className={`rounded-lg border border-red-900/60 bg-red-950/30 text-sm ${
        compact ? "p-4" : "p-6"
      } ${className}`.trim()}
    >
      <p className="font-medium text-red-300">{title}</p>
      {message && (
        <p className="mt-1 text-[var(--color-text-secondary)]">{message}</p>
      )}
      {onRetry && (
        <button
          type="button"
          onClick={onRetry}
          disabled={retrying}
          className="mt-4 rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm font-medium text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-accent)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent)] disabled:cursor-not-allowed disabled:opacity-50"
        >
          {retrying ? "Retrying…" : retryLabel}
        </button>
      )}
    </div>
  );
}

/**
 * Shared empty-state panel for list pages. Distinguishes a page with no data
 * yet (first-run, with an optional next action) from a filtered view that
 * matches nothing (no-results), and reuses the same visual language as
 * ErrorState.
 */
export type EmptyStateVariant = "first-run" | "no-results" | "error";

export interface EmptyStateAction {
  label: string;
  onClick?: () => void;
  href?: string;
}

export interface EmptyStateProps {
  variant?: EmptyStateVariant;
  title: string;
  description?: string;
  /** Primary next action, e.g. "Track a contract". */
  action?: EmptyStateAction;
  /** Optional secondary action, e.g. "Clear filters" on no-results. */
  secondaryAction?: EmptyStateAction;
  compact?: boolean;
  className?: string;
}

const VARIANT_CLASSES: Record<EmptyStateVariant, string> = {
  "first-run": "border-[var(--color-border)] bg-[var(--color-surface)]",
  "no-results": "border-[var(--color-border)] bg-[var(--color-surface)]",
  error: "border-red-900/60 bg-red-950/30",
};

function ActionButton({
  action,
  primary,
}: {
  action: EmptyStateAction;
  primary: boolean;
}) {
  const className = primary
    ? "rounded-md bg-[var(--color-accent)] px-3 py-1.5 text-sm font-medium text-white transition-opacity hover:opacity-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent)]"
    : "rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm font-medium text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-accent)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent)]";
  if (action.href) {
    return (
      <a href={action.href} className={className}>
        {action.label}
      </a>
    );
  }
  return (
    <button type="button" onClick={action.onClick} className={className}>
      {action.label}
    </button>
  );
}

/**
 * Empty state for list pages. Use `first-run` when no data exists yet (with a
 * next action), `no-results` when filters match nothing, and `error` for
 * failures. Announces itself to assistive tech via role="status" (or
 * role="alert" for errors).
 */
export function EmptyState({
  variant = "first-run",
  title,
  description,
  action,
  secondaryAction,
  compact = false,
  className = "",
}: EmptyStateProps) {
  return (
    <div
      role={variant === "error" ? "alert" : "status"}
      data-variant={variant}
      className={`rounded-lg border text-sm ${
        compact ? "p-4" : "p-8"
      } text-center ${VARIANT_CLASSES[variant]} ${className}`.trim()}
    >
      <p className="font-medium text-[var(--color-text-primary)]">{title}</p>
      {description && (
        <p className="mt-1 text-[var(--color-text-secondary)]">{description}</p>
      )}
      {(action || secondaryAction) && (
        <div className="mt-4 flex flex-wrap items-center justify-center gap-2">
          {action && <ActionButton action={action} primary />}
          {secondaryAction && (
            <ActionButton action={secondaryAction} primary={false} />
          )}
        </div>
      )}
    </div>
  );
}

/** Human-readable message for an unknown thrown value. */
export function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error && err.message ? err.message : fallback;
}
