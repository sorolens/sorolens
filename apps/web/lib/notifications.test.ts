import { describe, expect, it } from "vitest";
import { buildSubscriptionRequest } from "./notifications";

const contract = "cabqgaydambqgaydambqgaydambqgaydambqgaydambqgaydambqgck3";

describe("buildSubscriptionRequest", () => {
  it("sends a routing key for PagerDuty", () => {
    expect(
      buildSubscriptionRequest({ contractId: contract, channel: "pagerduty", destination: " key123 " }),
    ).toEqual({
      request: { contract_id: contract.toUpperCase(), channel_type: "pagerduty", routing_key: "key123" },
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
        },
      });
    }
  });

  it("requires https for Slack and Discord but allows http webhooks", () => {
    expect(
      buildSubscriptionRequest({ contractId: contract, channel: "slack", destination: "http://hooks.slack.com/x" }),
    ).toEqual({ error: expect.stringMatching(/https/) });
    expect(
      buildSubscriptionRequest({ contractId: contract, channel: "webhook", destination: "http://internal:8080/hook" }),
    ).toHaveProperty("request");
  });

  it("rejects missing fields and bad URLs", () => {
    expect(buildSubscriptionRequest({ contractId: "", channel: "webhook", destination: "https://x.io" })).toHaveProperty("error");
    expect(buildSubscriptionRequest({ contractId: contract, channel: "discord", destination: "" })).toHaveProperty("error");
    expect(buildSubscriptionRequest({ contractId: contract, channel: "webhook", destination: "not a url" })).toHaveProperty("error");
    expect(buildSubscriptionRequest({ contractId: contract, channel: "webhook", destination: "ftp://x.io" })).toHaveProperty("error");
  });
});
