import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import NotificationsPage from "./page";
import type { AlertSubscription, MonitoredContract } from "@/lib/types";

// ── Mock next/link ──────────────────────────────────────────────────────────
vi.mock("next/link", () => ({
  default: ({
    children,
    href,
  }: {
    children: React.ReactNode;
    href: string;
  }) => <a href={href}>{children}</a>,
}));

// ── Mock API & User modules ─────────────────────────────────────────────────
const mockListSubscriptions = vi.fn();
const mockCreateSubscription = vi.fn();
const mockDeleteSubscription = vi.fn();
const mockSendTestNotification = vi.fn();
const mockListMonitoredContracts = vi.fn();
const mockEnsureSession = vi.fn();
const mockGetUserId = vi.fn();

vi.mock("@/lib/api", () => ({
  ApiError: class ApiError extends Error {
    constructor(
      public status: number,
      message: string
    ) {
      super(message);
    }
  },
  listSubscriptions: (...args: unknown[]) => mockListSubscriptions(...args),
  createSubscription: (...args: unknown[]) => mockCreateSubscription(...args),
  deleteSubscription: (...args: unknown[]) => mockDeleteSubscription(...args),
  sendTestNotification: (...args: unknown[]) =>
    mockSendTestNotification(...args),
  listMonitoredContracts: (...args: unknown[]) =>
    mockListMonitoredContracts(...args),
}));

vi.mock("@/lib/user", () => ({
  ensureSession: () => mockEnsureSession(),
  getUserId: () => mockGetUserId(),
}));

const mockContracts: MonitoredContract[] = [
  {
    contract_id: "CABQGAYDAMBQGAYDAMBQGAYDAMBQGAYDAMBQGAYDAMBQGAYDAMBQGCK3",
    name: "Stellar Liquidity Pool",
    network: "testnet",
    owner: "G123",
    status: "healthy",
    last_check: null,
    check_interval: 60,
    registered_at: "2026-10-18T00:00:00Z",
    updated_at: "2026-10-18T00:00:00Z",
  },
];

const mockSubs: AlertSubscription[] = [
  {
    id: "sub_1",
    contract_id: "CABQGAYDAMBQGAYDAMBQGAYDAMBQGAYDAMBQGAYDAMBQGAYDAMBQGCK3",
    channel_type: "slack",
    webhook_url: "https://hooks.slack.com/***",
    has_routing_key: false,
    severity_filter: "Warning",
    created_at: "2026-10-18T00:00:00Z",
    updated_at: "2026-10-18T00:00:00Z",
  },
];

describe("NotificationsPage", () => {
  beforeEach(() => {
    mockListSubscriptions.mockReset();
    mockCreateSubscription.mockReset();
    mockDeleteSubscription.mockReset();
    mockSendTestNotification.mockReset();
    mockListMonitoredContracts.mockReset();
    mockEnsureSession.mockReset();
    mockGetUserId.mockReset();

    mockGetUserId.mockReturnValue("");
    mockEnsureSession.mockResolvedValue({
      user_id: "user_bootstrapped_123",
      token: "sls_token_abc",
      expires_at: new Date(Date.now() + 3600_000).toISOString(),
    });
    mockListSubscriptions.mockResolvedValue({ subscriptions: [] });
    mockListMonitoredContracts.mockResolvedValue({
      contracts: mockContracts,
      next_cursor: "",
    });
  });

  afterEach(() => {
    cleanup();
  });

  it("bootstraps user ID on a first visit and loads subscriptions", async () => {
    render(<NotificationsPage />);

    await waitFor(() => {
      expect(mockEnsureSession).toHaveBeenCalledTimes(1);
      expect(mockListSubscriptions).toHaveBeenCalledWith(
        "user_bootstrapped_123"
      );
    });
  });

  it("renders active subscriptions with severity badges", async () => {
    mockListSubscriptions.mockResolvedValueOnce({ subscriptions: mockSubs });

    render(<NotificationsPage />);

    await waitFor(() => {
      expect(screen.getByText("Active channels")).toBeInTheDocument();
      expect(screen.getByText("Warning")).toBeInTheDocument();
      expect(screen.getByText("Slack")).toBeInTheDocument();
    });
  });

  it("selects a contract via ContractPicker and saves chosen severity", async () => {
    const newSub: AlertSubscription = {
      id: "sub_2",
      contract_id: mockContracts[0].contract_id,
      channel_type: "discord",
      webhook_url: "https://discord.com/api/webhooks/***",
      has_routing_key: false,
      severity_filter: "Info",
      created_at: "2026-10-18T00:00:00Z",
      updated_at: "2026-10-18T00:00:00Z",
    };
    mockCreateSubscription.mockResolvedValueOnce(newSub);

    render(<NotificationsPage />);

    // Open contract picker and pick the contract
    fireEvent.click(screen.getByTestId("contract-picker-trigger"));
    await waitFor(() =>
      expect(screen.getByText("Stellar Liquidity Pool")).toBeInTheDocument()
    );
    fireEvent.click(screen.getByTestId("contract-picker-item"));

    // Select Discord channel
    fireEvent.click(screen.getByRole("radio", { name: "Discord" }));

    // Select Info severity
    fireEvent.click(screen.getByRole("radio", { name: "Info" }));

    // Type destination
    const destInput = screen.getByPlaceholderText(
      "https://discord.com/api/webhooks/…"
    );
    fireEvent.change(destInput, {
      target: { value: "https://discord.com/api/webhooks/1/secret" },
    });

    // Submit form
    fireEvent.click(
      screen.getByRole("button", { name: "Add Discord channel" })
    );

    await waitFor(() => {
      expect(mockCreateSubscription).toHaveBeenCalledWith(
        {
          contract_id: mockContracts[0].contract_id,
          channel_type: "discord",
          webhook_url: "https://discord.com/api/webhooks/1/secret",
          severity_filter: "Info",
        },
        "user_bootstrapped_123"
      );
    });

    // Verify it is added to the table
    await waitFor(() => {
      expect(screen.getAllByText("Info").length).toBeGreaterThanOrEqual(2);
    });
  });

  it("sends a test notification successfully", async () => {
    mockSendTestNotification.mockResolvedValueOnce({
      success: true,
      message: "Test notification sent successfully to Slack.",
    });

    render(<NotificationsPage />);

    // Pick contract
    fireEvent.click(screen.getByTestId("contract-picker-trigger"));
    await waitFor(() =>
      expect(screen.getByText("Stellar Liquidity Pool")).toBeInTheDocument()
    );
    fireEvent.click(screen.getByTestId("contract-picker-item"));

    // Enter destination
    const destInput = screen.getByPlaceholderText(
      "https://hooks.slack.com/services/…"
    );
    fireEvent.change(destInput, {
      target: { value: "https://hooks.slack.com/services/T/B/X" },
    });

    // Click Send test notification
    const testBtn = screen.getByTestId("test-send-btn");
    fireEvent.click(testBtn);

    await waitFor(() => {
      expect(mockSendTestNotification).toHaveBeenCalledWith({
        contractId: mockContracts[0].contract_id,
        channel: "slack",
        destination: "https://hooks.slack.com/services/T/B/X",
        severity: "Critical",
      });
      expect(
        screen.getByText("Test notification sent successfully to Slack.")
      ).toBeInTheDocument();
    });
  });

  it("displays an error when test notification fails", async () => {
    mockSendTestNotification.mockRejectedValueOnce(
      new Error("Upstream webhook returned status 404")
    );

    render(<NotificationsPage />);

    // Pick contract
    fireEvent.click(screen.getByTestId("contract-picker-trigger"));
    await waitFor(() =>
      expect(screen.getByText("Stellar Liquidity Pool")).toBeInTheDocument()
    );
    fireEvent.click(screen.getByTestId("contract-picker-item"));

    // Enter destination
    const destInput = screen.getByPlaceholderText(
      "https://hooks.slack.com/services/…"
    );
    fireEvent.change(destInput, {
      target: { value: "https://hooks.slack.com/services/T/B/X" },
    });

    // Click Send test notification
    const testBtn = screen.getByTestId("test-send-btn");
    fireEvent.click(testBtn);

    await waitFor(() => {
      expect(
        screen.getByText("Upstream webhook returned status 404")
      ).toBeInTheDocument();
    });
  });

  it("removes a subscription", async () => {
    mockListSubscriptions.mockResolvedValueOnce({ subscriptions: mockSubs });
    mockDeleteSubscription.mockResolvedValueOnce(undefined);

    render(<NotificationsPage />);

    await waitFor(() => {
      expect(screen.getByText("Remove")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Remove"));

    await waitFor(() => {
      expect(mockDeleteSubscription).toHaveBeenCalledWith(
        "sub_1",
        "user_bootstrapped_123"
      );
      expect(screen.queryByText("Remove")).not.toBeInTheDocument();
      expect(
        screen.getByText(/No notification channels yet/)
      ).toBeInTheDocument();
    });
  });
});
