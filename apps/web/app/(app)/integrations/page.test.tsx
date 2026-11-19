import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const listIntegrations = vi.fn();
const testIntegration = vi.fn();

vi.mock("@/lib/integrations", () => ({
  listIntegrations: () => listIntegrations(),
  testIntegration: (id: string) => testIntegration(id),
}));

import IntegrationsPage from "./page";

const SLACK = {
  id: "slack" as const,
  name: "Slack",
  description: "Post watchdog alerts into a Slack workspace.",
  install_url: "https://api.slack.com/apps",
  setup_steps: ["Create the app", "Set SLACK_WEBHOOK_URL"],
  configured: true,
};

const DISCORD = {
  id: "discord" as const,
  name: "Discord",
  description: "Relay watchdog alerts to a Discord channel.",
  install_url: "https://discord.com/developers/applications",
  setup_steps: ["Invite the bot"],
  configured: false,
};

describe("IntegrationsPage", () => {
  beforeEach(() => {
    listIntegrations.mockReset();
    listIntegrations.mockResolvedValue([SLACK, DISCORD]);
    testIntegration.mockReset();
  });

  afterEach(() => {
    cleanup();
  });

  it("documents each integration with its setup steps and status", async () => {
    render(<IntegrationsPage />);

    await waitFor(() =>
      expect(screen.getByTestId("integration-slack")).toBeTruthy()
    );
    expect(screen.getByRole("heading", { name: "Integrations" })).toBeTruthy();
    expect(screen.getByText("Create the app")).toBeTruthy();
    expect(screen.getByText("Set SLACK_WEBHOOK_URL")).toBeTruthy();
    expect(screen.getByTestId("integration-slack-status").textContent).toBe(
      "Configured"
    );
    expect(screen.getByTestId("integration-discord-status").textContent).toBe(
      "Not configured"
    );
    expect(screen.getByRole("link", { name: "Install Slack" })).toHaveAttribute(
      "href",
      "https://api.slack.com/apps"
    );
  });

  it("sends a test message and reports success", async () => {
    testIntegration.mockResolvedValueOnce({
      ok: true,
      detail: "test message delivered",
    });

    render(<IntegrationsPage />);
    await waitFor(() =>
      expect(screen.getByTestId("integration-slack-test")).toBeTruthy()
    );

    fireEvent.click(screen.getByTestId("integration-slack-test"));

    await waitFor(() => expect(testIntegration).toHaveBeenCalledWith("slack"));
    expect(
      screen.getByTestId("integration-slack-feedback").textContent
    ).toContain("test message delivered");
  });

  it("surfaces a failed test send", async () => {
    testIntegration.mockRejectedValueOnce(
      new Error("integration is not configured: set DISCORD_WEBHOOK_URL")
    );

    render(<IntegrationsPage />);
    await waitFor(() =>
      expect(screen.getByTestId("integration-discord-test")).toBeTruthy()
    );

    fireEvent.click(screen.getByTestId("integration-discord-test"));

    await waitFor(() =>
      expect(
        screen.getByTestId("integration-discord-feedback").textContent
      ).toContain("not configured")
    );
  });
});
