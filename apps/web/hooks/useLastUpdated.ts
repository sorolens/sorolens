"use client";

import { useSyncExternalStore } from "react";
import {
  getLastUpdated,
  getServerLastUpdated,
  subscribeLastUpdated,
  type LastUpdatedSnapshot,
} from "@/lib/lastUpdated";

/**
 * Subscribe to the shared "last updated" store that `lib/api.ts` writes to
 * after every successful fetch.
 *
 * `useSyncExternalStore` is used deliberately: during hydration React renders
 * the server snapshot (never updated), which matches the server HTML, and only
 * the post-hydration render shows the recorded timestamp.
 */
export function useLastUpdated(): LastUpdatedSnapshot {
  return useSyncExternalStore(
    subscribeLastUpdated,
    getLastUpdated,
    getServerLastUpdated
  );
}
