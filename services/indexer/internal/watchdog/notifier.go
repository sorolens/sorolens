package watchdog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/sorolens/sorolens/services/indexer/internal/webhooksig"
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
	// SigningSecret is the subscription's whsec_... key. Deliveries are signed
	// with it; a subscription without one is skipped rather than sent unsigned.
	SigningSecret string
}

// maxDeliveryAttempts is one send plus one retry, as before.
const maxDeliveryAttempts = 2

// DispatchAlerts queries matching subscriptions for a critical alert
// and fires HTTP POSTs to their webhook URLs.
//
// Every delivery is signed with the subscription's signing secret: the request
// carries X-Sorolens-Timestamp and X-Sorolens-Signature (see webhooksig and
// docs/webhooks.md). It retries once on 5xx responses, logs and skips on 4xx.
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
	if len(subs) == 0 {
		return
	}

	body, err := alertPayload(alert)
	if err != nil {
		logger.Error("dispatch alerts: marshal payload", "err", err, "contract_id", alert.ContractID)
		return
	}

	// One client for the whole fan-out; *http.Client is safe for concurrent use.
	client := &http.Client{Timeout: 10 * time.Second}

	for _, sub := range subs {
		if sub.SeverityFilter != "Critical" && sub.SeverityFilter != alert.Severity {
			continue
		}
		go deliver(ctx, client, sub, body, logger)
	}
}

// alertPayload is the JSON body delivered to a subscriber. It is marshalled
// once per alert so every subscription (and every retry) signs identical bytes.
func alertPayload(alert Alert) ([]byte, error) {
	return json.Marshal(map[string]any{
		"contract_id":  alert.ContractID,
		"severity":     alert.Severity,
		"message":      alert.Message,
		"timestamp":    alert.Timestamp.Format(time.RFC3339),
		"explorer_url": fmt.Sprintf("https://sorobanexplorer.com/transaction/%s", alert.TxHash),
	})
}

// deliver signs and POSTs one alert to one subscription, retrying once on 5xx.
//
// The timestamp and signature are computed once and every attempt re-sends the
// exact signed bytes, so a retry is still verifiable by the receiver.
func deliver(ctx context.Context, client *http.Client, sub AlertSubscription, body []byte, logger *slog.Logger) {
	if sub.SigningSecret == "" {
		// Never deliver an unsigned payload: a receiver that enforces HMAC
		// would reject it, and silently sending one would defeat the feature.
		logger.Error("dispatch alerts: subscription has no signing secret; skipping delivery",
			"subscription_id", sub.ID)
		return
	}

	timestamp := time.Now().Unix()
	signature := webhooksig.Sign(sub.SigningSecret, timestamp, body)

	var lastStatus int
	for attempt := 0; attempt < maxDeliveryAttempts; attempt++ {
		req, err := signedRequest(ctx, sub.WebhookURL, body, timestamp, signature)
		if err != nil {
			logger.Error("dispatch alerts: create request", "err", err, "subscription_id", sub.ID)
			return
		}

		resp, err := client.Do(req)
		if err != nil {
			logger.Error("dispatch alerts: request failed", "err", err, "subscription_id", sub.ID)
			return
		}
		status := resp.StatusCode
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()

		if status < http.StatusInternalServerError {
			if status >= http.StatusBadRequest {
				logger.Error("dispatch alerts: client error", "status", status, "subscription_id", sub.ID)
			}
			return
		}
		lastStatus = status
	}

	logger.Error("dispatch alerts: retry also failed", "status", lastStatus, "subscription_id", sub.ID)
}

// signedRequest builds the POST with the body and the two signature headers.
func signedRequest(ctx context.Context, url string, body []byte, timestamp int64, signature string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(webhooksig.TimestampHeader, strconv.FormatInt(timestamp, 10))
	req.Header.Set(webhooksig.SignatureHeader, webhooksig.SignatureHeaderValue(timestamp, signature))
	return req, nil
}
