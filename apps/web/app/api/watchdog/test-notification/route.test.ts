import { beforeEach, describe, expect, it, vi } from "vitest";
import { POST } from "./route";

const originalFetch = globalThis.fetch;

function createRequest(body: unknown): Request {
  return new Request("http://localhost/api/watchdog/test-notification", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

describe("POST /api/watchdog/test-notification", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("returns 400 when contract_id is missing", async () => {
    const res = await POST(
      createRequest({
        channel_type: "slack",
        destination: "https://hooks.slack.com/services/x",
      })
    );
    expect(res.status).toBe(400);
    const data = await res.json();
    expect(data.error).toContain("contract_id is required");
  });

  it("returns 400 when channel_type is invalid", async () => {
    const res = await POST(
      createRequest({
        contract_id: "CA123",
        channel_type: "telegram",
        destination: "https://example.com",
      })
    );
    expect(res.status).toBe(400);
    const data = await res.json();
    expect(data.error).toContain("channel_type must be one of");
  });

  it("returns 400 when destination is missing", async () => {
    const res = await POST(
      createRequest({
        contract_id: "CA123",
        channel_type: "discord",
        destination: "",
      })
    );
    expect(res.status).toBe(400);
    const data = await res.json();
    expect(data.error).toContain("required");
  });

  it("dispatches test notification to Slack successfully", async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
    });
    globalThis.fetch = mockFetch;

    const res = await POST(
      createRequest({
        contract_id: "CA123",
        channel_type: "slack",
        destination: "https://hooks.slack.com/services/T/B/X",
        severity: "Critical",
      })
    );

    expect(res.status).toBe(200);
    const data = await res.json();
    expect(data.success).toBe(true);
    expect(data.message).toContain("Slack");
    expect(mockFetch).toHaveBeenCalledWith(
      "https://hooks.slack.com/services/T/B/X",
      expect.objectContaining({
        method: "POST",
        headers: { "Content-Type": "application/json" },
      })
    );
  });

  it("dispatches test notification to Discord successfully", async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 204,
    });
    globalThis.fetch = mockFetch;

    const res = await POST(
      createRequest({
        contract_id: "CA123",
        channel_type: "discord",
        destination: "https://discord.com/api/webhooks/1/2",
        severity: "Warning",
      })
    );

    expect(res.status).toBe(200);
    const data = await res.json();
    expect(data.success).toBe(true);
    expect(data.message).toContain("Discord");
  });

  it("dispatches test notification to PagerDuty successfully", async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 202,
    });
    globalThis.fetch = mockFetch;

    const res = await POST(
      createRequest({
        contract_id: "CA123",
        channel_type: "pagerduty",
        destination: "pd-routing-key-12345",
        severity: "Info",
      })
    );

    expect(res.status).toBe(200);
    const data = await res.json();
    expect(data.success).toBe(true);
    expect(mockFetch).toHaveBeenCalledWith(
      "https://events.pagerduty.com/v2/enqueue",
      expect.objectContaining({ method: "POST" })
    );
  });

  it("handles upstream channel errors gracefully", async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 404,
      statusText: "Not Found",
      text: vi.fn().mockResolvedValue("Webhook not found"),
    });
    globalThis.fetch = mockFetch;

    const res = await POST(
      createRequest({
        contract_id: "CA123",
        channel_type: "webhook",
        destination: "https://example.com/not-found",
      })
    );

    expect(res.status).toBe(400);
    const data = await res.json();
    expect(data.success).toBe(false);
    expect(data.error).toContain("404");
  });

  it("handles network failure gracefully", async () => {
    const mockFetch = vi
      .fn()
      .mockRejectedValue(new Error("Connection refused"));
    globalThis.fetch = mockFetch;

    const res = await POST(
      createRequest({
        contract_id: "CA123",
        channel_type: "webhook",
        destination: "https://invalid-host.example",
      })
    );

    expect(res.status).toBe(502);
    const data = await res.json();
    expect(data.success).toBe(false);
    expect(data.error).toContain("Connection refused");
  });
});
