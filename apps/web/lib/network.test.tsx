import { expect, it } from "vitest";
import { networkFilter } from "./network";

it("omits the network filter when all networks are selected", () => {
  expect(networkFilter("all")).toBeUndefined();
});

it("keeps the selected network when filtering by testnet", () => {
  expect(networkFilter("testnet")).toBe("testnet");
});

import { renderHook } from "@testing-library/react";
import { NetworkProvider, useNetwork } from "./network";
import { clearSettings, DEFAULT_SETTINGS, saveSettings } from "./settings";

it("starts with the user's saved network", () => {
  saveSettings({ ...DEFAULT_SETTINGS, defaultNetwork: "mainnet" });

  try {
    const { result, unmount } = renderHook(() => useNetwork(), {
      wrapper: NetworkProvider,
    });

    expect(result.current.network).toBe("mainnet");
    unmount();
  } finally {
    clearSettings();
  }
});
