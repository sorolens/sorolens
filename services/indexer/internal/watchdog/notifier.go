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

// AlertSubscriptionStore is the local interface for alert subscription storage.
type AlertSubscriptionStore interface {
	ListByContract(ctx context.Context, contractID string) ([]AlertSubscription, error)
	CreateDelivery(ctx context.Context, d WebhookDelivery) error
	UpdateDelivery(ctx context.Context, d WebhookDelivery) error
	UpdateDeliveryStatus(ctx context.Context, subscriptionID string, status string, at time.Time) error
}

// AlertSubscription is the local mirror of the store model.
type AlertSubscription struct {
	ID                 string
	ContractID         string
	WebhookURL         string
	SeverityFilter     string
	LastDeliveryStatus *string
	LastDeliveryAt     *time.Time
}

// DispatchAlerts queries matching subscriptions for an alert, creates a persistent delivery record,
// attempts immediate delivery, and schedules retries with exponential backoff on failure.
func DispatchAlerts(ctx context.Context, alert Alert, subStore AlertSubscriptionStore, logger *slog.Logger) {
	if alert.Severity != "Critical" {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}

	subs, err := subStore.ListByContract(ctx, alert.ContractID)
	if err != nil {
		logger.Error("dispatch alerts: list subscriptions", "err", err, "contract_id", alert.ContractID)
		return
	}

	cfg := LoadRetryConfig()
	dStore, ok := subStore.(DeliveryStore)
	var worker *RetryWorker
	if ok {
		worker = NewRetryWorker(dStore, logger, cfg, nil)
	}

	for _, sub := range subs {
		if sub.SeverityFilter != "Critical" && sub.SeverityFilter != alert.Severity {
			continue
		}
		go func(s AlertSubscription) {
			payload := map[string]any{
				"contract_id":  alert.ContractID,
				"severity":     alert.Severity,
				"message":      alert.Message,
				"timestamp":    alert.Timestamp.Format(time.RFC3339),
				"explorer_url": fmt.Sprintf("https://sorobanexplorer.com/transaction/%s", alert.TxHash),
			}
			body, err := json.Marshal(payload)
			if err != nil {
				logger.Error("dispatch alerts: marshal payload", "err", err, "subscription_id", s.ID)
				return
			}

			now := time.Now().UTC()
			deliveryID := fmt.Sprintf("del_%d", now.UnixNano())
			delivery := WebhookDelivery{
				ID:             deliveryID,
				SubscriptionID: s.ID,
				Payload:        string(body),
				Status:         "pending",
				Attempt:        0,
				MaxAttempts:    cfg.MaxAttempts,
				NextAttemptAt:  now,
				CreatedAt:      now,
				UpdatedAt:      now,
			}

			if err := subStore.CreateDelivery(ctx, delivery); err != nil {
				logger.Error("dispatch alerts: create delivery record", "err", err, "subscription_id", s.ID)
				// Even if DB creation fails, try inline dispatch
				req, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.WebhookURL, bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				client := &http.Client{Timeout: 10 * time.Second}
				resp, rErr := client.Do(req)
				if rErr == nil {
					resp.Body.Close()
				}
				return
			}

			if worker != nil {
				_ = worker.ProcessSingleDelivery(ctx, delivery, s.WebhookURL)
			}
		}(sub)
	}
}
