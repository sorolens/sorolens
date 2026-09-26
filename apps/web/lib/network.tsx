"use client";

import { createContext, useContext, useState } from "react";

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
  return (
    <NetworkContext.Provider value={{ network, setNetwork }}>
      {children}
    </NetworkContext.Provider>
  );
}
