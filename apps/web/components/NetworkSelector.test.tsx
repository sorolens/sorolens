/**
 * Tests for apps/web/components/NetworkSelector.tsx (issue #627).
 *
 * The selector writes into the network context, so a probe component reads
 * the context back out to observe the change the way list views would.
 */

// Registers jest-dom matchers with vitest, including their types.
import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { NetworkSelector } from "./NetworkSelector";
import { ALL_NETWORKS, NetworkProvider, useNetwork } from "@/lib/network";

function NetworkProbe() {
  const { network } = useNetwork();
  return <output data-testid="network-probe">{network}</output>;
}

function renderSelector() {
  return render(
    <NetworkProvider>
      <NetworkProbe />
      <NetworkSelector />
    </NetworkProvider>
  );
}

beforeEach(() => {
  window.localStorage.clear();
});

afterEach(() => {
  cleanup();
  window.localStorage.clear();
});

describe("NetworkSelector", () => {
  it("labels the select for assistive technology", () => {
    renderSelector();

    const select = screen.getByRole("combobox", { name: "Network" });
    expect(select).toHaveAttribute("id", "network-selector");
    expect(select).toHaveAccessibleName("Network");
  });

  it("defaults to the unfiltered network", () => {
    renderSelector();

    const select = screen.getByRole<HTMLSelectElement>("combobox", {
      name: "Network",
    });
    expect(select).toHaveValue(ALL_NETWORKS);
    expect(screen.getByRole("option", { name: "All networks" })).toBeVisible();
    for (const name of ["testnet", "mainnet", "futurenet"]) {
      expect(screen.getByRole("option", { name })).toBeVisible();
    }
  });

  it("publishes a selection to the network context", () => {
    renderSelector();

    fireEvent.change(screen.getByRole("combobox", { name: "Network" }), {
      target: { value: "futurenet" },
    });

    expect(screen.getByTestId("network-probe")).toHaveTextContent("futurenet");
    expect(
      screen.getByRole<HTMLSelectElement>("combobox", { name: "Network" }).value
    ).toBe("futurenet");
  });

  it("applies the default network saved in Settings on mount", async () => {
    window.localStorage.setItem(
      "sorolens_settings",
      JSON.stringify({ defaultNetwork: "mainnet" })
    );
    renderSelector();

    // The provider reads the saved preference in an effect after mount.
    expect(await screen.findByTestId("network-probe")).toHaveTextContent(
      "mainnet"
    );
  });
});
