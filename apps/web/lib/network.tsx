"use client";

import { createContext, useContext, useEffect, useState } from "react";

import { loadSettings } from "./settings";

/** Sentinel value meaning "do not filter by network". */
export const ALL_NETWORKS = "all";

/** Networks the dashboard can filter by. */
export const NETWORKS = [
  ALL_NETWORKS,
  "testnet",
  "mainnet",
  "futurenet",
] as const;

interface NetworkContextValue {
  /** Currently selected network, or ALL_NETWORKS. */
  network: string;
  setNetwork: (network: string) => void;
}

const NetworkContext = createContext<NetworkContextValue>({
  network: ALL_NETWORKS,
  setNetwork: () => {},
});

/** Read the currently selected network from context. */
export function useNetwork(): NetworkContextValue {
  return useContext(NetworkContext);
}

/**
 * Converts the selection into an API query value: undefined when "all" is
 * selected so callers omit the ?network= parameter entirely.
 */
export function networkFilter(network: string): string | undefined {
  return network === ALL_NETWORKS ? undefined : network;
}

export function NetworkProvider({ children }: { children: React.ReactNode }) {
  const [network, setNetwork] = useState<string>(ALL_NETWORKS);

  // Apply the default network saved on /settings once, on first mount. Reading
  // it in an effect rather than during render keeps server and client markup in
  // step, and the provider does not remount while the user navigates.
  useEffect(() => {
    const stored = loadSettings().defaultNetwork;
    if (stored && stored !== ALL_NETWORKS) {
      setNetwork(stored);
    }
  }, []);

  return (
    <NetworkContext.Provider value={{ network, setNetwork }}>
      {children}
    </NetworkContext.Provider>
  );
}
