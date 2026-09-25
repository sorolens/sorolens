package watchdog_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/sorolens/sorolens/services/indexer/internal/watchdog"
)

type mockDeliveryStore struct {
	mu            sync.Mutex
	subs          map[string]watchdog.AlertSubscription
	deliveries    map[string]watchdog.WebhookDelivery
	deliveryOrder []string
}

func newMockDeliveryStore() *mockDeliveryStore {
	return &mockDeliveryStore{
		subs:       make(map[string]watchdog.AlertSubscription),
		deliveries: make(map[string]watchdog.WebhookDelivery),
	}
}

func (m *mockDeliveryStore) CreateDelivery(_ context.Context, d watchdog.WebhookDelivery) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deliveries[d.ID] = d
	m.deliveryOrder = append(m.deliveryOrder, d.ID)
	return nil
}

func (m *mockDeliveryStore) UpdateDelivery(_ context.Context, d watchdog.WebhookDelivery) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deliveries[d.ID] = d
	return nil
}

func (m *mockDeliveryStore) GetPendingDeliveries(_ context.Context, limit int) ([]watchdog.WebhookDelivery, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []watchdog.WebhookDelivery
	now := time.Now()
	for _, id := range m.deliveryOrder {
		d := m.deliveries[id]
		if d.Status == "pending" && (d.NextAttemptAt.Before(now) || d.NextAttemptAt.Equal(now)) {
			out = append(out, d)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (m *mockDeliveryStore) GetByID(_ context.Context, id string) (watchdog.AlertSubscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sub, ok := m.subs[id]
	if !ok {
		return watchdog.AlertSubscription{}, fmt.Errorf("not found")
	}
	return sub, nil
}

func (m *mockDeliveryStore) UpdateDeliveryStatus(_ context.Context, subscriptionID string, status string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	sub, ok := m.subs[subscriptionID]
	if !ok {
		return fmt.Errorf("not found")
	}
	sub.LastDeliveryStatus = &status
	sub.LastDeliveryAt = &at
	m.subs[subscriptionID] = sub
	return nil
}

func TestCalculateNextBackoff_DefaultSchedule(t *testing.T) {
	schedule := watchdog.DefaultBackoffSchedule

	expected := []time.Duration{
		1 * time.Minute,
		5 * time.Minute,
		15 * time.Minute,
		1 * time.Hour,
		6 * time.Hour,
	}

	for i, exp := range expected {
		attempt := i + 1
		got := watchdog.CalculateNextBackoff(attempt, schedule)
		if got != exp {
			t.Errorf("attempt %d: expected %s, got %s", attempt, exp, got)
		}
	}

	// Attempt beyond schedule length caps at last interval (6h)
	gotOver := watchdog.CalculateNextBackoff(10, schedule)
	if gotOver != 6*time.Hour {
		t.Errorf("attempt 10: expected 6h, got %s", gotOver)
	}
}

func TestCalculateNextAttempt_MaxAttemptsExhausted(t *testing.T) {
	now := time.Now().UTC()
	schedule := watchdog.DefaultBackoffSchedule
	maxAttempts := 5

	nextAt, hasMore := watchdog.CalculateNextAttempt(4, maxAttempts, schedule, now)
	if !hasMore {
		t.Errorf("attempt 4 should have more retries")
	}
	expectedNext := now.Add(1 * time.Hour)
	if !nextAt.Equal(expectedNext) {
		t.Errorf("expected %v, got %v", expectedNext, nextAt)
	}

	_, hasMoreMax := watchdog.CalculateNextAttempt(5, maxAttempts, schedule, now)
	if hasMoreMax {
		t.Errorf("attempt 5 should exhaust retries")
	}
}

func TestLoadRetryConfig_EnvironmentVariables(t *testing.T) {
	os.Setenv("WEBHOOK_MAX_RETRIES", "3")
	os.Setenv("WEBHOOK_BACKOFF_SCHEDULE", "10s,30s,2m")
	defer func() {
		os.Unsetenv("WEBHOOK_MAX_RETRIES")
		os.Unsetenv("WEBHOOK_BACKOFF_SCHEDULE")
	}()

	cfg := watchdog.LoadRetryConfig()
	if cfg.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts=3, got %d", cfg.MaxAttempts)
	}
	if len(cfg.BackoffSchedule) != 3 {
		t.Fatalf("expected 3 durations, got %d", len(cfg.BackoffSchedule))
	}
	if cfg.BackoffSchedule[0] != 10*time.Second || cfg.BackoffSchedule[1] != 30*time.Second || cfg.BackoffSchedule[2] != 2*time.Minute {
		t.Errorf("unexpected schedule: %v", cfg.BackoffSchedule)
	}
}

func TestRetryWorker_ProcessSingleDelivery_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ms := newMockDeliveryStore()
	sub := watchdog.AlertSubscription{
		ID:             "sub_1",
		ContractID:     "C123",
		WebhookURL:     srv.URL,
		SeverityFilter: "Critical",
	}
	ms.subs[sub.ID] = sub

	now := time.Now().UTC()
	del := watchdog.WebhookDelivery{
		ID:             "del_1",
		SubscriptionID: sub.ID,
		Payload:        `{"alert":"test"}`,
		Status:         "pending",
		Attempt:        0,
		MaxAttempts:    5,
		NextAttemptAt:  now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	_ = ms.CreateDelivery(context.Background(), del)

	worker := watchdog.NewRetryWorker(ms, nil, watchdog.LoadRetryConfig(), nil)
	err := worker.ProcessSingleDelivery(context.Background(), del, srv.URL)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	d := ms.deliveries["del_1"]
	if d.Status != "success" {
		t.Errorf("expected status 'success', got %q", d.Status)
	}
	if d.Attempt != 1 {
		t.Errorf("expected attempt 1, got %d", d.Attempt)
	}

	updatedSub, _ := ms.GetByID(context.Background(), sub.ID)
	if updatedSub.LastDeliveryStatus == nil || *updatedSub.LastDeliveryStatus != "success" {
		t.Errorf("expected sub LastDeliveryStatus='success', got %v", updatedSub.LastDeliveryStatus)
	}
}

func TestRetryWorker_ProcessSingleDelivery_MockFailingEndpoint_Retries(t *testing.T) {
	attemptCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ms := newMockDeliveryStore()
	sub := watchdog.AlertSubscription{
		ID:             "sub_2",
		ContractID:     "C456",
		WebhookURL:     srv.URL,
		SeverityFilter: "Critical",
	}
	ms.subs[sub.ID] = sub

	now := time.Now().UTC()
	del := watchdog.WebhookDelivery{
		ID:             "del_2",
		SubscriptionID: sub.ID,
		Payload:        `{"alert":"test"}`,
		Status:         "pending",
		Attempt:        0,
		MaxAttempts:    3,
		NextAttemptAt:  now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	_ = ms.CreateDelivery(context.Background(), del)

	cfg := watchdog.RetryConfig{
		MaxAttempts:     3,
		BackoffSchedule: []time.Duration{1 * time.Minute, 5 * time.Minute, 15 * time.Minute},
		HTTPTimeout:     5 * time.Second,
	}
	worker := watchdog.NewRetryWorker(ms, nil, cfg, nil)

	// Attempt 1: fails, schedules next attempt
	err1 := worker.ProcessSingleDelivery(context.Background(), del, srv.URL)
	if err1 == nil {
		t.Fatalf("expected attempt 1 to fail")
	}

	del1 := ms.deliveries["del_2"]
	if del1.Status != "pending" {
		t.Errorf("attempt 1 status: expected 'pending', got %q", del1.Status)
	}
	if del1.Attempt != 1 {
		t.Errorf("attempt 1 count: expected 1, got %d", del1.Attempt)
	}

	// Attempt 2: fails
	err2 := worker.ProcessSingleDelivery(context.Background(), del1, srv.URL)
	if err2 == nil {
		t.Fatalf("expected attempt 2 to fail")
	}

	del2 := ms.deliveries["del_2"]
	if del2.Status != "pending" {
		t.Errorf("attempt 2 status: expected 'pending', got %q", del2.Status)
	}
	if del2.Attempt != 2 {
		t.Errorf("attempt 2 count: expected 2, got %d", del2.Attempt)
	}

	// Attempt 3: max retries reached -> marked failed
	err3 := worker.ProcessSingleDelivery(context.Background(), del2, srv.URL)
	if err3 == nil {
		t.Fatalf("expected attempt 3 to fail")
	}

	del3 := ms.deliveries["del_2"]
	if del3.Status != "failed" {
		t.Errorf("attempt 3 status: expected 'failed', got %q", del3.Status)
	}
	if del3.Attempt != 3 {
		t.Errorf("attempt 3 count: expected 3, got %d", del3.Attempt)
	}

	updatedSub, _ := ms.GetByID(context.Background(), sub.ID)
	if updatedSub.LastDeliveryStatus == nil || *updatedSub.LastDeliveryStatus != "failed" {
		t.Errorf("expected sub LastDeliveryStatus='failed', got %v", updatedSub.LastDeliveryStatus)
	}
}
