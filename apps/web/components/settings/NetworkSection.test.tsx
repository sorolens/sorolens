/**
 * Tests for apps/web/components/settings/NetworkSection.tsx (issue #765).
 *
 * The card is controlled: the page owns the value and the status. It must
 * publish every change upward and offer exactly the networks the dashboard
 * can filter by.
 */

import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { NetworkSection } from "./NetworkSection";
import { ALL_NETWORKS, NETWORKS } from "@/lib/network";

afterEach(cleanup);

/** The select's accessible name comes from its SettingsRow label. */
function networkSelect(): HTMLSelectElement {
  return screen.getByRole<HTMLSelectElement>("combobox", {
    name: "Default network",
  });
}

function renderSection(
  overrides: {
    value?: string;
    status?: "idle" | "saved" | "error";
  } = {}
) {
  const onChange = vi.fn();
  const onSave = vi.fn(() => true);
  render(
    <NetworkSection
      value={overrides.value ?? ALL_NETWORKS}
      status={overrides.status ?? "idle"}
      onChange={onChange}
      onSave={onSave}
    />
  );
  return { onChange, onSave };
}

describe("NetworkSection", () => {
  it("offers every selectable network with the ALL sentinel labelled", () => {
    renderSection();

    const select = networkSelect();
    const options = Array.from(select.options).map((o) => o.value);
    expect(options).toEqual([...NETWORKS]);

    expect(screen.getByRole("option", { name: "All networks" })).toBeVisible();
    for (const network of NETWORKS.filter((n) => n !== ALL_NETWORKS)) {
      expect(screen.getByRole("option", { name: network })).toBeVisible();
    }
  });

  it("reflects the value it is given", () => {
    renderSection({ value: "mainnet" });

    expect(networkSelect().value).toBe("mainnet");
  });

  it("publishes a selection without saving it", () => {
    const { onChange, onSave } = renderSection();

    fireEvent.change(networkSelect(), {
      target: { value: "futurenet" },
    });

    expect(onChange).toHaveBeenCalledWith("futurenet");
    expect(onSave).not.toHaveBeenCalled();
  });

  it("saves through the card's Save button", () => {
    const { onSave } = renderSection();

    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(onSave).toHaveBeenCalledTimes(1);
  });

  it("surfaces a failed save", () => {
    renderSection({ status: "error" });

    expect(screen.getByRole("status")).toHaveTextContent(
      "Could not save — storage unavailable"
    );
  });
});
