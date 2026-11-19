"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";

/**
 * How a state change reaches the URL.
 *
 * - "push" adds a history entry, so browser Back/Forward steps between the
 *   previous and the new state (applying a filter, going to the next page).
 * - "replace" rewrites the current entry (restoring defaults, clearing a
 *   param whose value reverted to the default).
 */
export type NavigateMode = "push" | "replace";

export interface QueryStateSpec<T> {
  /**
   * Parse the page's shareable state out of the current query params. Must
   * be total: any unknown, missing or malformed value falls back to a
   * default, never to an exception.
   */
  parse: (params: URLSearchParams) => T;
  /**
   * Serialize state to params. Returns the COMPLETE param set the page
   * owns: keys that are absent (or undefined/empty) are removed from the
   * URL. Params the page does not own are preserved untouched.
   */
  serialize: (state: T) => Record<string, string | undefined>;
}

function toQueryString(entries: Record<string, string | undefined>): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(entries)) {
    if (value !== undefined && value !== "") params.set(key, value);
  }
  return params.toString();
}

/**
 * Keeps a piece of page state in the URL so a view is deep-linkable and
 * survives reload (issue #618).
 *
 * - The initial state is parsed from the URL on mount, so a reload or a
 *   shared link restores the exact view.
 * - `setQueryState` writes the serialized state back with router.push, so
 *   Back/Forward moves between views.
 * - When the user navigates (back, forward, or a link to the same page with
 *   different params), the new query string is parsed back into state.
 *
 * Params the page does not own are preserved: serialize() only describes
 * this page's own params.
 */
export function useQueryState<T>(
  spec: QueryStateSpec<T>
): [T, (update: T | ((prev: T) => T), mode?: NavigateMode) => void] {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const paramsKey = searchParams ? searchParams.toString() : "";

  const [state, setState] = useState<T>(() =>
    spec.parse(new URLSearchParams(paramsKey))
  );

  // Mirror of state and spec for use inside callbacks/effects without
  // re-subscribing (the spec is typically an inline object).
  const stateRef = useRef(state);
  const specRef = useRef(spec);
  specRef.current = spec;

  // The query string this hook itself last navigated to; when that
  // navigation reports back through useSearchParams, the state has already
  // been set locally and must not be re-parsed.
  const lastWrittenRef = useRef<string | null>(null);

  // URL -> state: any navigation this hook did not initiate (Back,
  // Forward, a link) re-parses the query string into state.
  useEffect(() => {
    if (lastWrittenRef.current === paramsKey) {
      lastWrittenRef.current = null;
      return;
    }
    const parsed = specRef.current.parse(new URLSearchParams(paramsKey));
    const current = specRef.current;
    // Skip when the parsed state serializes to the same params: avoids
    // replacing the state object (and re-triggering data effects) when
    // nothing shareable actually changed.
    if (
      toQueryString(current.serialize(parsed)) ===
      toQueryString(current.serialize(stateRef.current))
    ) {
      return;
    }
    stateRef.current = parsed;
    setState(parsed);
  }, [paramsKey]);

  const setQueryState = useCallback(
    (update: T | ((prev: T) => T), mode: NavigateMode = "push") => {
      const prev = stateRef.current;
      const next =
        typeof update === "function" ? (update as (p: T) => T)(prev) : update;

      // A no-op update (the updater returned the previous state) must not
      // navigate: several callers express "nothing to do" that way, and
      // rewriting the URL would ping-pong and pollute history.
      if (next === prev) return;

      // Preserve params this page does not own.
      const owned = new Set(Object.keys(specRef.current.serialize(next)));
      const params = new URLSearchParams(paramsKey);
      for (const key of Array.from(params.keys())) {
        if (owned.has(key)) params.delete(key);
      }
      const nextParams = new URLSearchParams(params.toString());
      for (const [key, value] of Object.entries(
        specRef.current.serialize(next)
      )) {
        if (value !== undefined && value !== "") nextParams.set(key, value);
      }

      const qs = nextParams.toString();
      const target = qs ? `${pathname}?${qs}` : pathname;
      lastWrittenRef.current = qs;
      if (mode === "push") {
        router.push(target, { scroll: false });
      } else {
        router.replace(target, { scroll: false });
      }
      stateRef.current = next;
      setState(next);
    },
    [router, pathname, paramsKey]
  );

  return [state, setQueryState];
}
