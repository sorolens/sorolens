import type { ChannelType, CreateSubscriptionRequest } from "./types";

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
  form: SubscriptionForm,
): { error: string } | { request: CreateSubscriptionRequest } {
  const contractId = form.contractId.trim().toUpperCase();
  const destination = form.destination.trim();
  if (!contractId) return { error: "Choose a contract." };
  if (!destination) return { error: `${CHANNELS[form.channel].destinationLabel} is required.` };

  switch (form.channel) {
    case "pagerduty":
      return {
        request: { contract_id: contractId, channel_type: "pagerduty", routing_key: destination },
      };
    case "slack":
    case "discord":
      if (!isURL(destination, ["https:"])) {
        return { error: `${CHANNELS[form.channel].destinationLabel} must be an https URL.` };
      }
      break;
    default:
      if (!isURL(destination, ["http:", "https:"])) {
        return { error: "Webhook URL must be an http(s) URL." };
      }
  }
  return {
    request: { contract_id: contractId, channel_type: form.channel, webhook_url: destination },
  };
}
