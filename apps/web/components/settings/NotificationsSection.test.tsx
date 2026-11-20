/**
 * Tests for apps/web/components/settings/NotificationsSection.tsx (issue #765).
 *
 * The card is controlled by the settings page, which validates the address
 * with `isValidEmail` and passes the resulting error down. These tests pin
 * that contract: every edit publishes a complete preference object, and a
 * rejected address is announced rather than silently dropped.
 */

import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { NotificationsSection } from "./NotificationsSection";
import {
  DEFAULT_SETTINGS,
  NOTIFICATION_CHANNELS,
  type NotificationPreferences,
} from "@/lib/settings";

afterEach(cleanup);

const PREFERENCES: NotificationPreferences = {
  email: "dev@example.com",
  channels: {
    critical_alerts: true,
    health_changes: true,
    weekly_digest: false,
  },
};

function renderSection(
  overrides: {
    preferences?: NotificationPreferences;
    error?: string | null;
    status?: "idle" | "saved" | "error";
  } = {}
) {
  const onChange = vi.fn();
  const onSave = vi.fn(() => true);
  render(
    <NotificationsSection
      preferences={overrides.preferences ?? PREFERENCES}
      error={overrides.error ?? null}
      status={overrides.status ?? "idle"}
      onChange={onChange}
      onSave={onSave}
    />
  );
  return { onChange, onSave };
}

describe("NotificationsSection", () => {
  it("renders the saved address and every channel toggle", () => {
    renderSection();

    expect(screen.getByLabelText("Email address")).toHaveValue(
      "dev@example.com"
    );
    for (const channel of NOTIFICATION_CHANNELS) {
      expect(
        screen.getByRole("checkbox", { name: channel.label })
      ).toBeInTheDocument();
    }
    expect(
      screen.getByRole("checkbox", { name: "Critical alerts" })
    ).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: "Weekly digest" })
    ).not.toBeChecked();
  });

  it("publishes an email edit", () => {
    const { onChange } = renderSection();

    fireEvent.change(screen.getByLabelText("Email address"), {
      target: { value: "ops@example.org" },
    });

    expect(onChange).toHaveBeenCalledWith({
      ...PREFERENCES,
      email: "ops@example.org",
    });
  });

  it("publishes a channel toggle without dropping the other channels", () => {
    const { onChange } = renderSection();

    fireEvent.click(screen.getByRole("checkbox", { name: "Critical alerts" }));

    expect(onChange).toHaveBeenCalledWith({
      ...PREFERENCES,
      channels: {
        critical_alerts: false,
        health_changes: true,
        weekly_digest: false,
      },
    });
  });

  it("flags the field and announces a rejected address", () => {
    renderSection({ error: "Enter a valid email address, or leave it blank." });

    const input = screen.getByLabelText("Email address");
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(input).toHaveAttribute(
      "aria-describedby",
      "settings-notification-error"
    );

    const alert = screen.getByRole("alert");
    expect(alert).toHaveAttribute(
      "data-testid",
      "settings-notifications-error"
    );
    expect(alert).toHaveTextContent(
      "Enter a valid email address, or leave it blank."
    );
  });

  it("leaves the field unflagged when there is no error", () => {
    renderSection();

    const input = screen.getByLabelText("Email address");
    expect(input).not.toHaveAttribute("aria-invalid");
    expect(input).not.toHaveAttribute("aria-describedby");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("saves through the card's Save button", () => {
    const { onSave } = renderSection();

    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(onSave).toHaveBeenCalledTimes(1);
  });

  it("reports a failed save", () => {
    renderSection({ status: "error" });

    expect(screen.getByRole("status")).toHaveTextContent(
      "Could not save — storage unavailable"
    );
  });

  it("accepts the defaults when the stored blob had no email", () => {
    renderSection({ preferences: DEFAULT_SETTINGS.notifications });

    expect(screen.getByLabelText("Email address")).toHaveValue("");
    expect(
      screen.getByRole("checkbox", { name: "Weekly digest" })
    ).not.toBeChecked();
  });
});
