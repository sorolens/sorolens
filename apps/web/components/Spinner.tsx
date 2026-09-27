import type { HTMLAttributes } from "react";

export type SpinnerSize = "sm" | "md" | "lg";

export interface SpinnerProps extends HTMLAttributes<HTMLDivElement> {
  /** Visual size of the spinner. Defaults to "md". */
  size?: SpinnerSize;
}

const sizeClasses: Record<SpinnerSize, string> = {
  sm: "h-4 w-4",
  md: "h-6 w-6",
  lg: "h-8 w-8",
};

/**
 * Reusable loading spinner. Renders a rotating ring drawn with a 2px border;
 * the ring colour is the current text colour (`border-current`), so control it
 * where you use it with a `text-*` token, e.g. `text-[var(--color-accent)]`.
 */
export function Spinner({
  size = "md",
  className = "",
  ...props
}: SpinnerProps) {
  return (
    <div
      role="status"
      aria-label="Loading"
      className={[
        "animate-spin rounded-full border-2 border-current border-t-transparent",
        sizeClasses[size],
        className,
      ]
        .filter(Boolean)
        .join(" ")}
      {...props}
    />
  );
}
