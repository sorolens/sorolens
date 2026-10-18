import { describe, expect, it } from "vitest";
import { buildSubscriptionRequest, formatTestPayload } from "./notifications";

const contract = "cabqgaydambqgaydambqgaydambqgaydambqgaydambqgaydambqgck3";

describe("buildSubscriptionRequest", () => {
  it("sends a routing key for PagerDuty and defaults severity to Critical", () => {
    expect(
      buildSubscriptionRequest({
        contractId: contract,
        channel: "pagerduty",
        destination: " key123 ",
      })
    ).toEqual({
      request: {
        contract_id: contract.toUpperCase(),
        channel_type: "pagerduty",
        routing_key: "key123",
        severity_filter: "Critical",
      },
    });
  });

  it("saves a chosen severity filter", () => {
    expect(
      buildSubscriptionRequest({
        contractId: contract,
        channel: "slack",
        destination: "https://hooks.slack.com/services/x",
        severity: "Warning",
      })
    ).toEqual({
      request: {
        contract_id: contract.toUpperCase(),
        channel_type: "slack",
        webhook_url: "https://hooks.slack.com/services/x",
        severity_filter: "Warning",
      },
    });

    expect(
      buildSubscriptionRequest({
        contractId: contract,
        channel: "webhook",
        destination: "https://example.com/alerts",
        severity: "Info",
      })
    ).toEqual({
      request: {
        contract_id: contract.toUpperCase(),
        channel_type: "webhook",
        webhook_url: "https://example.com/alerts",
        severity_filter: "Info",
      },
    });
  });

  it("sends a webhook URL for Slack, Discord and webhook", () => {
    for (const channel of ["slack", "discord", "webhook"] as const) {
      const res = buildSubscriptionRequest({
        contractId: contract,
        channel,
        destination: "https://hooks.example.com/x",
      });
      expect(res).toEqual({
        request: {
          contract_id: contract.toUpperCase(),
          channel_type: channel,
          webhook_url: "https://hooks.example.com/x",
          severity_filter: "Critical",
        },
      });
    }
  });

  it("requires https for Slack and Discord but allows http webhooks", () => {
    expect(
      buildSubscriptionRequest({
        contractId: contract,
        channel: "slack",
        destination: "http://hooks.slack.com/x",
      })
    ).toEqual({ error: expect.stringMatching(/https/) });
    expect(
      buildSubscriptionRequest({
        contractId: contract,
        channel: "webhook",
        destination: "http://internal:8080/hook",
      })
    ).toHaveProperty("request");
  });

  it("rejects missing fields and bad URLs", () => {
    expect(
      buildSubscriptionRequest({
        contractId: "",
        channel: "webhook",
        destination: "https://x.io",
      })
    ).toHaveProperty("error");
    expect(
      buildSubscriptionRequest({
        contractId: contract,
        channel: "discord",
        destination: "",
      })
    ).toHaveProperty("error");
    expect(
      buildSubscriptionRequest({
        contractId: contract,
        channel: "webhook",
        destination: "not a url",
      })
    ).toHaveProperty("error");
    expect(
      buildSubscriptionRequest({
        contractId: contract,
        channel: "webhook",
        destination: "ftp://x.io",
      })
    ).toHaveProperty("error");
  });
});

describe("formatTestPayload", () => {
  it("formats Slack Block Kit payload", () => {
    const payload = formatTestPayload({
      contractId: contract,
      channel: "slack",
      destination: "https://hooks.slack.com/services/T/B/X",
      severity: "Warning",
    });
    expect(payload.url).toBe("https://hooks.slack.com/services/T/B/X");
    expect(payload.body.text).toContain("Warning");
    expect(Array.isArray(payload.body.blocks)).toBe(true);
  });

  it("formats Discord embed payload", () => {
    const payload = formatTestPayload({
      contractId: contract,
      channel: "discord",
      destination: "https://discord.com/api/webhooks/1/2",
      severity: "Critical",
    });
    expect(payload.url).toBe("https://discord.com/api/webhooks/1/2");
    expect(Array.isArray(payload.body.embeds)).toBe(true);
    const embeds = payload.body.embeds as Record<string, unknown>[];
    expect(embeds[0].color).toBe(0xef4444);
  });

  it("formats PagerDuty Events API v2 payload with routing key", () => {
    const payload = formatTestPayload({
      contractId: contract,
      channel: "pagerduty",
      destination: "key123",
      severity: "Critical",
    });
    expect(payload.url).toBe("https://events.pagerduty.com/v2/enqueue");
    expect(payload.body.routing_key).toBe("key123");
    expect(payload.body.event_action).toBe("trigger");
  });

  it("formats generic webhook payload", () => {
    const payload = formatTestPayload({
      contractId: contract,
      channel: "webhook",
      destination: "https://example.com/hook",
      severity: "Info",
    });
    expect(payload.url).toBe("https://example.com/hook");
    expect(payload.body.contract_id).toBe(contract.toUpperCase());
    expect(payload.body.severity).toBe("Info");
    expect(payload.body.event_type).toBe("test_notification");
  });
});
