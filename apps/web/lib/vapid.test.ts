import { afterEach, expect, it, vi } from "vitest";
import webPush from "web-push";
import { sendPushNotification, type PushPayload } from "./vapid";

vi.mock("web-push", () => ({
  default: {
    setVapidDetails: vi.fn(),
    sendNotification: vi.fn().mockResolvedValue(undefined),
  },
}));

afterEach(() => {
  vi.unstubAllEnvs();
  vi.clearAllMocks();
});

it("sends a critical alert to the subscription with high urgency", async () => {
  vi.stubEnv("VAPID_PUBLIC_KEY", "test-public-key");
  vi.stubEnv("VAPID_PRIVATE_KEY", "test-private-key");

  const subscription = {
    endpoint: "https://example.com/push/test",
    keys: { auth: "test-auth", p256dh: "test-key" },
  };
  const payload: PushPayload = {
    title: "Contract alert",
    body: "Failure rate exceeded the threshold",
    timestamp: "2026-11-19T10:00:00Z",
    severity: "Critical",
    tag: "contract-123",
  };

  await sendPushNotification(subscription, payload);

  expect(webPush.sendNotification).toHaveBeenCalledWith(
    subscription,
    JSON.stringify(payload),
    expect.objectContaining({ urgency: "high", topic: "contract-123" })
  );
});

it("rejects before sending when the VAPID private key is missing", async () => {
  vi.stubEnv("VAPID_PUBLIC_KEY", "test-public-key");
  vi.stubEnv("VAPID_PRIVATE_KEY", undefined);

  const subscription = {
    endpoint: "https://example.com/push/test",
    keys: { auth: "test-auth", p256dh: "test-key" },
  };
  const payload: PushPayload = {
    title: "Contract alert",
    body: "A contract needs attention",
    timestamp: "2026-11-19T10:00:00Z",
  };

  await expect(sendPushNotification(subscription, payload)).rejects.toThrow(
    "VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY must be set"
  );
  expect(webPush.sendNotification).not.toHaveBeenCalled();
});
