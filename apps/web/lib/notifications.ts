import type { ChannelType, CreateSubscriptionRequest } from "./types";

export type SeverityFilter = "Critical" | "Warning" | "Info";

export const SEVERITIES: readonly SeverityFilter[] = [
  "Critical",
  "Warning",
  "Info",
];

/** Display metadata for each notification channel (issue #127). */
export const CHANNELS: Record<
  ChannelType,
  { label: string; destinationLabel: string; placeholder: string; help: string }
> = {
  slack: {
    label: "Slack",
    destinationLabel: "Slack incoming webhook URL",
    placeholder: "https://hooks.slack.com/services/…",
    help: "Posts a Block Kit message. Create an incoming webhook in your Slack app and paste its URL.",
  },
  discord: {
    label: "Discord",
    destinationLabel: "Discord webhook URL",
    placeholder: "https://discord.com/api/webhooks/…",
    help: "Posts an embed. Channel settings → Integrations → Webhooks → Copy URL.",
  },
  pagerduty: {
    label: "PagerDuty",
    destinationLabel: "Integration (routing) key",
    placeholder: "32-character Events API v2 key",
    help: "Triggers an incident through Events API v2. Add an Events API v2 integration to a service and paste its key.",
  },
  webhook: {
    label: "Webhook",
    destinationLabel: "Webhook URL",
    placeholder: "https://example.com/sorolens-alerts",
    help: "POSTs a generic JSON payload to any HTTP(S) endpoint.",
  },
};

export interface SubscriptionForm {
  contractId: string;
  channel: ChannelType;
  /** Webhook URL, or the routing key for PagerDuty. */
  destination: string;
  severity?: SeverityFilter;
}

function isURL(value: string, schemes: string[]): boolean {
  try {
    const u = new URL(value);
    return schemes.includes(u.protocol) && u.host !== "";
  } catch {
    return false;
  }
}

/**
 * Validates the form with the same rules as the API and returns either an
 * error message or the request body to send.
 */
export function buildSubscriptionRequest(
  form: SubscriptionForm
): { error: string } | { request: CreateSubscriptionRequest } {
  const contractId = form.contractId.trim().toUpperCase();
  const destination = form.destination.trim();
  const severity: SeverityFilter = form.severity || "Critical";

  if (!contractId) return { error: "Choose a contract." };
  if (!destination)
    return { error: `${CHANNELS[form.channel].destinationLabel} is required.` };

  switch (form.channel) {
    case "pagerduty":
      return {
        request: {
          contract_id: contractId,
          channel_type: "pagerduty",
          routing_key: destination,
          severity_filter: severity,
        },
      };
    case "slack":
    case "discord":
      if (!isURL(destination, ["https:"])) {
        return {
          error: `${CHANNELS[form.channel].destinationLabel} must be an https URL.`,
        };
      }
      break;
    default:
      if (!isURL(destination, ["http:", "https:"])) {
        return { error: "Webhook URL must be an http(s) URL." };
      }
  }
  return {
    request: {
      contract_id: contractId,
      channel_type: form.channel,
      webhook_url: destination,
      severity_filter: severity,
    },
  };
}

export interface FormattedNotification {
  url: string;
  body: Record<string, unknown>;
  headers: Record<string, string>;
}

/**
 * Formats a test notification for the specified channel matching the indexer's
 * delivery formats (services/indexer/internal/watchdog/notifier.go).
 */
export function formatTestPayload(params: {
  contractId: string;
  channel: ChannelType;
  destination: string;
  severity?: SeverityFilter | string;
}): FormattedNotification {
  const contractId = params.contractId.trim().toUpperCase();
  const severity = params.severity || "Critical";
  const now = new Date().toISOString();
  const destination = params.destination.trim();

  switch (params.channel) {
    case "slack":
      return {
        url: destination,
        headers: { "Content-Type": "application/json" },
        body: {
          text: `🚨 ${severity} alert test on ${contractId}: SoroLens Watchdog test notification`,
          blocks: [
            {
              type: "header",
              text: {
                type: "plain_text",
                text: `🚨 SoroLens Watchdog Test Alert (${severity})`,
                emoji: true,
              },
            },
            {
              type: "section",
              fields: [
                { type: "mrkdwn", text: `*Contract*\n\`${contractId}\`` },
                { type: "mrkdwn", text: `*Severity*\n${severity}` },
              ],
            },
            {
              type: "section",
              text: {
                type: "mrkdwn",
                text: "This is a test notification from SoroLens Watchdog to verify your alert delivery configuration.",
              },
            },
            {
              type: "context",
              elements: [{ type: "mrkdwn", text: `Timestamp: ${now}` }],
            },
          ],
        },
      };

    case "discord": {
      const color =
        severity === "Critical"
          ? 0xef4444
          : severity === "Warning"
            ? 0xf59e0b
            : 0x3b82f6;
      return {
        url: destination,
        headers: { "Content-Type": "application/json" },
        body: {
          embeds: [
            {
              title: `🚨 SoroLens Watchdog Test Alert (${severity})`,
              description:
                "This is a test notification from SoroLens Watchdog to verify your alert delivery configuration.",
              color,
              fields: [
                { name: "Contract", value: `\`${contractId}\``, inline: true },
                { name: "Severity", value: severity, inline: true },
              ],
              timestamp: now,
            },
          ],
        },
      };
    }

    case "pagerduty": {
      const isEndpoint = destination.startsWith("http");
      const url = isEndpoint
        ? destination
        : "https://events.pagerduty.com/v2/enqueue";
      const routingKey = isEndpoint ? "" : destination;
      const pdSeverity =
        severity === "Critical"
          ? "critical"
          : severity === "Warning"
            ? "warning"
            : "info";

      return {
        url,
        headers: { "Content-Type": "application/json" },
        body: {
          routing_key: routingKey,
          event_action: "trigger",
          dedup_key: `test-alert-${contractId}-${Date.now()}`,
          payload: {
            summary: `[TEST] ${severity} alert on ${contractId}: SoroLens Watchdog test notification`,
            source: "sorolens-watchdog",
            severity: pdSeverity,
            timestamp: now,
            custom_details: {
              contract_id: contractId,
              test: true,
            },
          },
        },
      };
    }

    case "webhook":
    default:
      return {
        url: destination,
        headers: { "Content-Type": "application/json" },
        body: {
          event_type: "test_notification",
          contract_id: contractId,
          severity,
          message:
            "SoroLens Watchdog test notification: your alert channel is configured correctly.",
          timestamp: now,
        },
      };
  }
}
