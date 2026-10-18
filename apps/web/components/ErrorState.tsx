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

/** Human-readable message for an unknown thrown value. */
export function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error && err.message ? err.message : fallback;
}
