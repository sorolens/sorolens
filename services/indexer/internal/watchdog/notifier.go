package watchdog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

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
}

// DispatchAlerts queries matching subscriptions for a critical alert
// and fires HTTP POSTs to their webhook URLs.
// It retries once on 5xx responses, logs and skips on 4xx.
// Each webhook request has a 10-second timeout.
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
		go func(s AlertSubscription) {
			payload := map[string]any{
				"contract_id": alert.ContractID,
				"severity":    alert.Severity,
				"message":     alert.Message,
				"timestamp":   alert.Timestamp.Format(time.RFC3339),
				"explorer_url": fmt.Sprintf("https://sorobanexplorer.com/transaction/%s", alert.TxHash),
			}
			body, err := json.Marshal(payload)
			if err != nil {
				logger.Error("dispatch alerts: marshal payload", "err", err, "subscription_id", s.ID)
				return
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.WebhookURL, bytes.NewReader(body))
			if err != nil {
				logger.Error("dispatch alerts: create request", "err", err, "subscription_id", s.ID)
				return
			}
			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				logger.Error("dispatch alerts: request failed", "err", err, "subscription_id", s.ID)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 500 {
				// Retry once on 5xx
				resp2, err := client.Do(req)
				if err != nil {
					logger.Error("dispatch alerts: retry failed", "err", err, "subscription_id", s.ID)
					return
				}
				defer resp2.Body.Close()
				if resp2.StatusCode >= 500 {
					logger.Error("dispatch alerts: retry also failed", "status", resp2.StatusCode, "subscription_id", s.ID)
				}
			} else if resp.StatusCode >= 400 {
				logger.Error("dispatch alerts: client error", "status", resp.StatusCode, "subscription_id", s.ID)
			}
		}(sub)
	}
}
