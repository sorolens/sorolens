import type { SWRConfiguration } from "swr";

/**
 * Shared SWR defaults for the dashboard.
 *
 * - `revalidateOnFocus` / `revalidateOnReconnect` refresh cached data when the
 *   user returns to the tab or the network comes back.
 * - `dedupingInterval` collapses simultaneous requests for the same key into a
 *   single network call.
 * - `keepPreviousData` keeps the last payload on screen while a new key
 *   revalidates, which is what makes stale-while-revalidate visible: the
 *   browser Network tab shows the background request while the table still
 *   renders the cached rows.
 *
 * Mounted once in `app/(app)/layout.tsx` so every dashboard page shares one
 * cache and the same revalidation policy.
 */
export const swrConfig: SWRConfiguration = {
  revalidateOnFocus: true,
  revalidateOnReconnect: true,
  dedupingInterval: 2000,
  keepPreviousData: true,
};
