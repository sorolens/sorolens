package watchdog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Notification channel types (alert_subscriptions.channel_type, issue #127).
const (
	ChannelWebhook   = "webhook"
	ChannelSlack     = "slack"
	ChannelDiscord   = "discord"
	ChannelPagerDuty = "pagerduty"
)

// PagerDutyEventsURL is the PagerDuty Events API v2 enqueue endpoint, used
// when a pagerduty subscription does not override webhook_url.
const PagerDutyEventsURL = "https://events.pagerduty.com/v2/enqueue"

// AlertSubscriptionStore is the local interface for alert subscription
// storage, kept separate from apps/api to avoid internal package imports.
type AlertSubscriptionStore interface {
	ListByContract(ctx context.Context, contractID string) ([]AlertSubscription, error)
}

// AlertSubscription is the local mirror of the store model.
type AlertSubscription struct {
	ID             string
	ContractID     string
	WebhookURL     string
	SeverityFilter string
	// ChannelType selects the payload format; empty means webhook.
	ChannelType string
	// RoutingKey is the PagerDuty integration key (pagerduty only).
	RoutingKey string
}

// explorerURL links an alert to the transaction that raised it.
func explorerURL(a Alert) string {
	return fmt.Sprintf("https://sorobanexplorer.com/transaction/%s", a.TxHash)
}

// FormatNotification renders the HTTP request body for one subscription's
// channel and returns it with the URL to POST it to.
func FormatNotification(sub AlertSubscription, alert Alert) (string, []byte, error) {
	var (
		payload any
		url     = sub.WebhookURL
	)
	switch sub.ChannelType {
	case "", ChannelWebhook:
		payload = webhookPayload(alert)
	case ChannelSlack:
		payload = slackPayload(alert)
	case ChannelDiscord:
		payload = discordPayload(alert)
	case ChannelPagerDuty:
		if sub.RoutingKey == "" {
			return "", nil, fmt.Errorf("pagerduty subscription %s has no routing key", sub.ID)
		}
		payload = pagerDutyPayload(alert, sub.RoutingKey)
		if url == "" {
			url = PagerDutyEventsURL
		}
	default:
		return "", nil, fmt.Errorf("unknown channel type %q", sub.ChannelType)
	}
	if url == "" {
		return "", nil, fmt.Errorf("subscription %s has no webhook url", sub.ID)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", nil, err
	}
	return url, body, nil
}

// webhookPayload is the generic JSON body (unchanged from the original
// webhook-only notifier so existing receivers keep working).
func webhookPayload(a Alert) map[string]any {
	return map[string]any{
		"contract_id":  a.ContractID,
		"severity":     a.Severity,
		"message":      a.Message,
		"timestamp":    a.Timestamp.Format(time.RFC3339),
		"explorer_url": explorerURL(a),
	}
}

// severityEmoji prefixes Slack text so severity is visible in notifications.
func severityEmoji(severity string) string {
	switch severity {
	case "Critical":
		return ":rotating_light:"
	case "Warning":
		return ":warning:"
	default:
		return ":information_source:"
	}
}

// truncate caps s at n runes, marking the cut with an ellipsis.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// slackPayload is a Slack incoming-webhook message using Block Kit. `text`
// is the fallback shown in push notifications.
func slackPayload(a Alert) map[string]any {
	title := fmt.Sprintf("%s %s alert", severityEmoji(a.Severity), a.Severity)
	return map[string]any{
		"text": fmt.Sprintf("%s on %s: %s", title, a.ContractID, truncate(a.Message, 200)),
		"blocks": []any{
			map[string]any{
				"type": "header",
				"text": map[string]any{"type": "plain_text", "text": truncate(fmt.Sprintf("%s alert", a.Severity), 150), "emoji": true},
			},
			map[string]any{
				"type": "section",
				"fields": []any{
					map[string]any{"type": "mrkdwn", "text": fmt.Sprintf("*Contract*\n`%s`", a.ContractID)},
					map[string]any{"type": "mrkdwn", "text": fmt.Sprintf("*Severity*\n%s %s", severityEmoji(a.Severity), a.Severity)},
				},
			},
			map[string]any{
				"type": "section",
				"text": map[string]any{"type": "mrkdwn", "text": truncate(a.Message, 2900)},
			},
			map[string]any{
				"type": "context",
				"elements": []any{
					map[string]any{"type": "mrkdwn", "text": fmt.Sprintf("Ledger %d · <!date^%d^{date_short_pretty} {time}|%s>",
						a.Ledger, a.Timestamp.Unix(), a.Timestamp.UTC().Format(time.RFC3339))},
				},
			},
			map[string]any{
				"type": "actions",
				"elements": []any{
					map[string]any{
						"type": "button",
						"text": map[string]any{"type": "plain_text", "text": "View transaction"},
						"url":  explorerURL(a),
					},
				},
			},
		},
	}
}

// Discord embed colors per severity.
const (
	discordColorCritical = 0xE5484D
	discordColorWarning  = 0xF5A524
	discordColorInfo     = 0x3B82F6
)

func discordColor(severity string) int {
	switch severity {
	case "Critical":
		return discordColorCritical
	case "Warning":
		return discordColorWarning
	default:
		return discordColorInfo
	}
}

// discordPayload is a Discord webhook message with one embed.
func discordPayload(a Alert) map[string]any {
	return map[string]any{
		"username": "Sorolens",
		"embeds": []any{
			map[string]any{
				"title":       truncate(fmt.Sprintf("%s alert", a.Severity), 256),
				"description": truncate(a.Message, 4000),
				"url":         explorerURL(a),
				"color":       discordColor(a.Severity),
				"timestamp":   a.Timestamp.UTC().Format(time.RFC3339),
				"fields": []any{
					map[string]any{"name": "Contract", "value": "`" + a.ContractID + "`"},
					map[string]any{"name": "Severity", "value": a.Severity, "inline": true},
					map[string]any{"name": "Ledger", "value": fmt.Sprintf("%d", a.Ledger), "inline": true},
				},
				"footer": map[string]any{"text": "Sorolens watchdog"},
			},
		},
	}
}

// pagerDutySeverity maps watchdog severities onto Events API v2 severities.
func pagerDutySeverity(severity string) string {
	switch severity {
	case "Critical":
		return "critical"
	case "Warning":
		return "warning"
	default:
		return "info"
	}
}

// pagerDutyPayload is a PagerDuty Events API v2 trigger event. The dedup key
// is derived from the alert so a re-delivered alert does not open a second
// incident.
func pagerDutyPayload(a Alert, routingKey string) map[string]any {
	return map[string]any{
		"routing_key":  routingKey,
		"event_action": "trigger",
		"dedup_key":    fmt.Sprintf("sorolens:%s:%s", a.ContractID, a.TxHash),
		"payload": map[string]any{
			"summary":   truncate(fmt.Sprintf("[%s] %s: %s", a.Severity, a.ContractID, a.Message), 1024),
			"source":    a.ContractID,
			"severity":  pagerDutySeverity(a.Severity),
			"timestamp": a.Timestamp.UTC().Format(time.RFC3339),
			"component": "sorolens-watchdog",
			"custom_details": map[string]any{
				"contract_id": a.ContractID,
				"message":     a.Message,
				"ledger":      a.Ledger,
				"tx_hash":     a.TxHash,
			},
		},
		"links": []any{
			map[string]any{"href": explorerURL(a), "text": "View transaction"},
		},
	}
}

// httpClient is the notifier's HTTP client; tests may replace it.
var httpClient = &http.Client{Timeout: 10 * time.Second}

// DispatchAlerts queries matching subscriptions for a critical alert and
// posts a channel-formatted notification to each one. It retries once on
// 5xx responses, logs and skips on 4xx. Each request has a 10-second
// timeout.
func DispatchAlerts(ctx context.Context, alert Alert, subStore AlertSubscriptionStore, logger *slog.Logger) {
	if alert.Severity != "Critical" {
		return
	}

	subs, err := subStore.ListByContract(ctx, alert.ContractID)
	if err != nil {
		logger.Error("dispatch alerts: list subscriptions", "err", err, "contract_id", alert.ContractID)
		return
	}

	for _, sub := range subs {
		if sub.SeverityFilter != "Critical" && sub.SeverityFilter != alert.Severity {
			continue
		}
		go deliver(ctx, sub, alert, logger)
	}
}

// deliver sends one notification, retrying once on a 5xx. Each attempt
// builds a fresh request so the body is re-sent in full.
func deliver(ctx context.Context, sub AlertSubscription, alert Alert, logger *slog.Logger) {
	url, body, err := FormatNotification(sub, alert)
	if err != nil {
		logger.Error("dispatch alerts: format", "err", err, "subscription_id", sub.ID, "channel", sub.ChannelType)
		return
	}
	for attempt := 1; attempt <= 2; attempt++ {
		status, err := post(ctx, url, body)
		switch {
		case err != nil:
			logger.Error("dispatch alerts: request failed", "err", err, "subscription_id", sub.ID, "attempt", attempt)
			return
		case status >= 500:
			logger.Error("dispatch alerts: server error", "status", status, "subscription_id", sub.ID, "attempt", attempt)
			continue
		case status >= 400:
			logger.Error("dispatch alerts: client error", "status", status, "subscription_id", sub.ID)
		}
		return
	}
}

func post(ctx context.Context, url string, body []byte) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}
