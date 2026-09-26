"use client";

import { usePathname, useSearchParams } from "next/navigation";
import { useEffect, useRef, useState } from "react";

/**
 * Thin progress bar shown at the top of the viewport during route transitions.
 *
 * Driven entirely by pathname + searchParams changes so it works with the
 * Next.js 15 App Router without any external library. The bar:
 *   1. Appears immediately on navigation start (via useEffect on the *previous*
 *      render's snapshot).
 *   2. Advances to ~80 % with a CSS transition while the new route loads.
 *   3. Jumps to 100 % and fades out once the new route has rendered.
 *
 * Uses --color-accent from the project design tokens so it respects the
 * light / dark theme automatically.
 */

type Phase = "idle" | "loading" | "completing";

const BAR_HEIGHT = "2px";
const TRANSITION_MS = 200;
const COMPLETE_HOLD_MS = 300;

export function NavigationProgress() {
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const [phase, setPhase] = useState<Phase>("idle");
  // Track the last-seen route key so we can detect when it changes.
  const prevRouteRef = useRef<string | null>(null);
  // Hold a reference to the completion timer so we can cancel on rapid
  // back-to-back navigations.
  const completeTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const routeKey = `${pathname}?${searchParams.toString()}`;

  useEffect(() => {
    // On the very first render there is no "previous" route — skip.
    if (prevRouteRef.current === null) {
      prevRouteRef.current = routeKey;
      return;
    }

    // Route has not changed — nothing to do.
    if (prevRouteRef.current === routeKey) {
      return;
    }

    prevRouteRef.current = routeKey;

    // Cancel any in-flight completion timer from a previous navigation.
    if (completeTimerRef.current !== null) {
      clearTimeout(completeTimerRef.current);
      completeTimerRef.current = null;
    }

    // New route rendered — complete the bar.
    setPhase("completing");

    completeTimerRef.current = setTimeout(() => {
      setPhase("idle");
      completeTimerRef.current = null;
    }, TRANSITION_MS + COMPLETE_HOLD_MS);

    return () => {
      if (completeTimerRef.current !== null) {
        clearTimeout(completeTimerRef.current);
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [routeKey]);

  // The bar is invisible when idle.
  if (phase === "idle") {
    return null;
  }

  const width = phase === "loading" ? "80%" : "100%";
  const opacity = phase === "completing" ? 0 : 1;

  return (
    <div
      role="progressbar"
      aria-label="Page loading"
      aria-valuenow={phase === "completing" ? 100 : 80}
      aria-valuemin={0}
      aria-valuemax={100}
      style={{
        position: "fixed",
        top: 0,
        left: 0,
        zIndex: 9999,
        height: BAR_HEIGHT,
        width,
        opacity,
        backgroundColor: "var(--color-accent)",
        transition: `width ${TRANSITION_MS}ms ease, opacity ${TRANSITION_MS}ms ease`,
        // Subtle glow that works in both themes.
        boxShadow: "0 0 6px 1px var(--color-accent)",
        pointerEvents: "none",
      }}
    />
  );
}
