"use client";

import { ALL_NETWORKS, NETWORKS, useNetwork } from "@/lib/network";

/**
 * Header network selector. Changing it filters every list view that reads the
 * network context (contracts, watchdog, and their detail pages).
 */
export function NetworkSelector() {
  const { network, setNetwork } = useNetwork();

  return (
    <label className="flex items-center gap-2 text-xs text-[var(--color-text-secondary)]">
      <span className="hidden sm:inline">Network</span>
      <select
        id="network-selector"
        aria-label="Network"
        value={network}
        onChange={(e) => setNetwork(e.target.value)}
        className="rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] px-2 py-1 text-xs text-[var(--color-text-primary)] focus:border-[var(--color-accent)] focus:outline-none"
      >
        {NETWORKS.map((n) => (
          <option key={n} value={n}>
            {n === ALL_NETWORKS ? "All networks" : n}
          </option>
        ))}
      </select>
    </label>
  );
}
