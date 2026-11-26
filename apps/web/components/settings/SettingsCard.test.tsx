/**
 * Tests for apps/web/components/settings/SettingsCard.tsx (issue #765).
 *
 * The card owns no state: the settings page passes `status` in and gets the
 * Save click back through `onSave`. These tests pin the contract every card
 * relies on — the labelled region, the save callback and the status line.
 */

import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SettingsCard, SettingsRow } from "./SettingsCard";

afterEach(cleanup);

function renderCard(
  overrides: Partial<Parameters<typeof SettingsCard>[0]> = {}
) {
  const onSave = vi.fn(() => true);
  const view = render(
    <SettingsCard
      testId="settings-example"
      title="Example card"
      description="What this card is for."
      status="idle"
      onSave={onSave}
      {...overrides}
    >
      <p>Card body</p>
    </SettingsCard>
  );
  return { onSave, container: view.container };
}

describe("SettingsCard", () => {
  it("renders the title, description and children inside a labelled region", () => {
    renderCard();

    const region = screen.getByRole("region", { name: "Example card" });
    expect(region).toHaveAttribute("data-testid", "settings-example");
    expect(screen.getByRole("heading", { name: "Example card" })).toBeVisible();
    expect(screen.getByText("What this card is for.")).toBeVisible();
    expect(screen.getByText("Card body")).toBeVisible();
  });

  it("calls onSave when the Save button is clicked", () => {
    const { onSave } = renderCard();

    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(onSave).toHaveBeenCalledTimes(1);
  });

  it("uses a custom save label when one is supplied", () => {
    renderCard({ saveLabel: "Save changes" });

    expect(screen.getByRole("button", { name: "Save changes" })).toBeVisible();
    expect(screen.queryByRole("button", { name: "Save" })).toBeNull();
  });

  it("shows no status line while idle", () => {
    const { container } = renderCard({ status: "idle" });

    expect(
      container.querySelector('[data-testid="settings-example-status"]')
    ).toBeNull();
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("announces a successful save", () => {
    renderCard({ status: "saved" });

    const status = screen.getByRole("status");
    expect(status).toHaveAttribute("data-testid", "settings-example-status");
    expect(status).toHaveTextContent("Saved");
  });

  it("reports a failed save instead of claiming success", () => {
    renderCard({ status: "error" });

    const status = screen.getByRole("status");
    expect(status).toHaveTextContent("Could not save — storage unavailable");
    expect(status).not.toHaveTextContent(/^Saved$/);
  });
});

describe("SettingsRow", () => {
  it("associates its label with the control it wraps", () => {
    render(
      <SettingsRow
        htmlFor="settings-row-input"
        title="Row title"
        description="Row description"
      >
        <input id="settings-row-input" type="checkbox" />
      </SettingsRow>
    );

    const label = screen.getByText("Row title");
    expect(label.tagName).toBe("LABEL");
    expect(label).toHaveAttribute("for", "settings-row-input");
    expect(screen.getByRole("checkbox")).toBeVisible();
    expect(screen.getByText("Row description")).toBeVisible();
  });
});
