package watchdog

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// WebhookDelivery mirror struct for watchdog package.
type WebhookDelivery struct {
	ID             string    `json:"id"`
	SubscriptionID string    `json:"subscription_id"`
	Payload        string    `json:"payload"`
	Status         string    `json:"status"` // pending | success | failed
	Attempt        int       `json:"attempt"`
	MaxAttempts    int       `json:"max_attempts"`
	NextAttemptAt  time.Time `json:"next_attempt_at"`
	ResponseCode   int       `json:"response_code,omitempty"`
	ErrorMessage   string    `json:"error_message,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// DefaultBackoffSchedule is 1 min -> 5 min -> 15 min -> 1 hr -> 6 hr.
var DefaultBackoffSchedule = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	1 * time.Hour,
	6 * time.Hour,
}

// CalculateNextBackoff calculates the backoff duration for a given attempt index (1-based).
func CalculateNextBackoff(attempt int, schedule []time.Duration) time.Duration {
	if len(schedule) == 0 {
		schedule = DefaultBackoffSchedule
	}
	idx := attempt - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(schedule) {
		idx = len(schedule) - 1
	}
	return schedule[idx]
}

// CalculateNextAttempt determines the next scheduled attempt timestamp and whether further retries remain.
func CalculateNextAttempt(attempt int, maxAttempts int, schedule []time.Duration, now time.Time) (time.Time, bool) {
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	if attempt >= maxAttempts {
		return now, false
	}
	backoff := CalculateNextBackoff(attempt, schedule)
	return now.Add(backoff), true
}

// RetryConfig holds environment-configurable options for webhook retries.
type RetryConfig struct {
	MaxAttempts     int
	BackoffSchedule []time.Duration
	PollInterval    time.Duration
	HTTPTimeout     time.Duration
}

// LoadRetryConfig loads retry settings from environment variables.
func LoadRetryConfig() RetryConfig {
	maxAttempts := 5
	if val := os.Getenv("WEBHOOK_MAX_RETRIES"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			maxAttempts = parsed
		}
	}

	schedule := DefaultBackoffSchedule
	if val := os.Getenv("WEBHOOK_BACKOFF_SCHEDULE"); val != "" {
		parts := strings.Split(val, ",")
		var parsed []time.Duration
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if d, err := time.ParseDuration(p); err == nil && d > 0 {
				parsed = append(parsed, d)
			}
		}
		if len(parsed) > 0 {
			schedule = parsed
		}
	}

	return RetryConfig{
		MaxAttempts:     maxAttempts,
		BackoffSchedule: schedule,
		PollInterval:    10 * time.Second,
		HTTPTimeout:     10 * time.Second,
	}
}

// DeliveryStore defines the database persistence interface needed by the retry worker.
type DeliveryStore interface {
	CreateDelivery(ctx context.Context, d WebhookDelivery) error
	UpdateDelivery(ctx context.Context, d WebhookDelivery) error
	GetPendingDeliveries(ctx context.Context, limit int) ([]WebhookDelivery, error)
	GetByID(ctx context.Context, id string) (AlertSubscription, error)
	UpdateDeliveryStatus(ctx context.Context, subscriptionID string, status string, at time.Time) error
}

// RetryWorker processes queued webhook deliveries and re-attempts failed deliveries.
type RetryWorker struct {
	store      DeliveryStore
	logger     *slog.Logger
	config     RetryConfig
	httpClient *http.Client
}

// NewRetryWorker constructs a RetryWorker instance.
func NewRetryWorker(st DeliveryStore, logger *slog.Logger, cfg RetryConfig, client *http.Client) *RetryWorker {
	if client == nil {
		client = &http.Client{Timeout: cfg.HTTPTimeout}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &RetryWorker{
		store:      st,
		logger:     logger,
		config:     cfg,
		httpClient: client,
	}
}

// ExecuteDelivery performs an HTTP POST request for a delivery attempt.
func (w *RetryWorker) ExecuteDelivery(ctx context.Context, webhookURL string, payload []byte) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(payload))
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("non-2xx response status: %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}

// ProcessSingleDelivery processes or re-attempts one WebhookDelivery.
func (w *RetryWorker) ProcessSingleDelivery(ctx context.Context, d WebhookDelivery, webhookURL string) error {
	now := time.Now().UTC()
	code, err := w.ExecuteDelivery(ctx, webhookURL, []byte(d.Payload))

	d.Attempt++
	d.ResponseCode = code
	d.UpdatedAt = now

	if err == nil {
		d.Status = "success"
		d.ErrorMessage = ""
		if updateErr := w.store.UpdateDelivery(ctx, d); updateErr != nil {
			w.logger.Error("failed to update delivery success state", "err", updateErr, "delivery_id", d.ID)
		}
		_ = w.store.UpdateDeliveryStatus(ctx, d.SubscriptionID, "success", now)
		return nil
	}

	errMsg := err.Error()
	d.ErrorMessage = errMsg
	w.logger.Warn("webhook delivery attempt failed",
		"delivery_id", d.ID,
		"subscription_id", d.SubscriptionID,
		"attempt", d.Attempt,
		"max_attempts", d.MaxAttempts,
		"err", errMsg,
	)

	nextAt, hasMore := CalculateNextAttempt(d.Attempt, d.MaxAttempts, w.config.BackoffSchedule, now)
	if !hasMore {
		d.Status = "failed"
		if updateErr := w.store.UpdateDelivery(ctx, d); updateErr != nil {
			w.logger.Error("failed to update delivery failed state", "err", updateErr, "delivery_id", d.ID)
		}
		_ = w.store.UpdateDeliveryStatus(ctx, d.SubscriptionID, "failed", now)
		w.logger.Error("webhook delivery exhausted retries",
			"delivery_id", d.ID,
			"subscription_id", d.SubscriptionID,
			"total_attempts", d.Attempt,
			"last_error", errMsg,
		)
		return err
	}

	d.Status = "pending"
	d.NextAttemptAt = nextAt
	if updateErr := w.store.UpdateDelivery(ctx, d); updateErr != nil {
		w.logger.Error("failed to update delivery pending state", "err", updateErr, "delivery_id", d.ID)
	}
	_ = w.store.UpdateDeliveryStatus(ctx, d.SubscriptionID, "pending", now)
	return err
}

// RunOnce polls and processes current due deliveries.
func (w *RetryWorker) RunOnce(ctx context.Context) (int, error) {
	deliveries, err := w.store.GetPendingDeliveries(ctx, 50)
	if err != nil {
		return 0, fmt.Errorf("get pending deliveries: %w", err)
	}

	processed := 0
	for _, d := range deliveries {
		sub, err := w.store.GetByID(ctx, d.SubscriptionID)
		if err != nil {
			w.logger.Error("subscription not found for delivery", "delivery_id", d.ID, "subscription_id", d.SubscriptionID, "err", err)
			d.Status = "failed"
			d.ErrorMessage = "subscription deleted"
			d.UpdatedAt = time.Now().UTC()
			_ = w.store.UpdateDelivery(ctx, d)
			continue
		}

		_ = w.ProcessSingleDelivery(ctx, d, sub.WebhookURL)
		processed++
	}

	return processed, nil
}
