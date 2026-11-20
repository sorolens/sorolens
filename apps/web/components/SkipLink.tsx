"use client";

import type { MouseEvent } from "react";

/** The id every page uses for its single main landmark. */
export const MAIN_CONTENT_ID = "main";

/**
 * Keyboard skip link.
 *
 * Rendered first in the root layout so it is the first focusable control on
 * every page. It stays visually hidden until focused, then moves focus to the
 * page's <main> landmark. The fragment href keeps it working without
 * JavaScript; the click handler only adds the focus move that a bare fragment
 * navigation does not guarantee.
 */
export function SkipLink({
  targetId = MAIN_CONTENT_ID,
}: {
  targetId?: string;
}) {
  function handleClick(event: MouseEvent<HTMLAnchorElement>) {
    const target = document.getElementById(targetId);
    if (!target) return;
    event.preventDefault();
    target.focus();
    target.scrollIntoView?.({ block: "start" });
  }

  return (
    <a
      href={`#${targetId}`}
      onClick={handleClick}
      className="sr-only focus:not-sr-only focus:absolute focus:left-4 focus:top-4 focus:z-50 focus:rounded-lg focus:border focus:border-[var(--color-accent)] focus:bg-[var(--color-bg-card)] focus:px-4 focus:py-2 focus:text-sm focus:font-semibold focus:text-[var(--color-text-primary)] focus:outline-2 focus:outline-offset-2 focus:outline-[var(--color-accent)]"
    >
      Skip to main content
    </a>
  );
}
