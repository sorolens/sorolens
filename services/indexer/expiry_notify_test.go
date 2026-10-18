package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/sorolens/sorolens/services/indexer/internal/pgstore"
	"github.com/sorolens/sorolens/services/indexer/internal/poller"
	"github.com/sorolens/sorolens/services/indexer/internal/watchdog"
)

// mockSubscriptionLister stands in for *pgstore.Store so the dispatch path can
// be exercised without Postgres (issue #510's mock-store requirement, read for
// the notifier side).
type mockSubscriptionLister struct {
	mu      sync.Mutex
	subs    []pgstore.AlertSubscription
	listErr error
	queried []string
}

func (m *mockSubscriptionLister) ListAlertSubscriptions(_ context.Context, contractID string) ([]pgstore.AlertSubscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queried = append(m.queried, contractID)
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.subs, nil
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func expiryAlert() poller.Alert {
	return poller.Alert{
		ContractID: "CDXQ",
		Severity:   "Critical",
		Message:    "2 storage entries expire within 5000 ledgers: 1 persistent, 1 temporary; earliest expiry at ledger 1000100",
		Ledger:     1_000_100,
		TxHash:     "expiry:CDXQ:deadbeefdeadbeef",
		Timestamp:  time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC),
	}
}

// drain waits for the fire-and-forget deliveries watchdog.DispatchAlerts spawns
// and reports how many bodies the endpoint received.
func drain(t *testing.T, hits *int, mu *sync.Mutex) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		n := *hits
		mu.Unlock()
		if n > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("no delivery arrived within 2s")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestExpiryNotifierDeliversSummaryToConfiguredWebhook proves the runner's
// summary reaches the webhooks a user registered, through the existing watchdog
// notifier rather than a parallel delivery implementation.
func TestExpiryNotifierDeliversSummaryToConfiguredWebhook(t *testing.T) {
	var (
		mu     sync.Mutex
		hits   int
		body   map[string]any
		srvURL string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var p map[string]any
		if err := json.Unmarshal(b, &p); err != nil {
			t.Errorf("webhook body is not JSON (%q)", b)
		}
		mu.Lock()
		hits++
		body = p
		mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()
	srvURL = srv.URL
	// httptest servers listen on loopback, which the default client refuses.
	t.Cleanup(watchdog.SetHTTPClient(&http.Client{Timeout: 5 * time.Second}))

	lister := &mockSubscriptionLister{subs: []pgstore.AlertSubscription{
		{ID: "sub-1", ContractID: "CDXQ", WebhookURL: srvURL, SeverityFilter: "Critical", ChannelType: "webhook"},
	}}
	newExpiryNotifier(lister, quietLogger()).Notify(context.Background(), expiryAlert())

	drain(t, &hits, &mu)

	mu.Lock()
	defer mu.Unlock()
	if body["contract_id"] != "CDXQ" || body["severity"] != "Critical" {
		t.Fatalf("payload = %v, want the expiry summary for CDXQ", body)
	}
	if msg, _ := body["message"].(string); msg == "" {
		t.Fatal("payload lost the summary message")
	}
	// The synthetic dedup key travels as the explorer link, as for every other
	// watchdog alert.
	if url, _ := body["explorer_url"].(string); url == "" {
		t.Fatalf("payload = %v, want an explorer URL", body)
	}
	// The subscription lookup is contract-scoped.
	if len(lister.queried) == 0 || lister.queried[0] != "CDXQ" {
		t.Fatalf("queried = %v, want CDXQ", lister.queried)
	}
}

// TestExpiryNotifierRespectsSubscriptionSeverityFilter pins the "configured
// webhooks" semantics: a subscription that only accepts Warning never receives
// the Critical expiry summary.
func TestExpiryNotifierRespectsSubscriptionSeverityFilter(t *testing.T) {
	var (
		mu   sync.Mutex
		hits int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	lister := &mockSubscriptionLister{subs: []pgstore.AlertSubscription{
		{ID: "sub-warn", ContractID: "CDXQ", WebhookURL: srv.URL, SeverityFilter: "Warning", ChannelType: "webhook"},
	}}
	newExpiryNotifier(lister, quietLogger()).Notify(context.Background(), expiryAlert())

	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if hits != 0 {
		t.Fatalf("deliveries = %d, want 0 for a Warning-only subscription", hits)
	}
}

// TestExpiryNotifierWithoutSubscriptionsIsANoOp keeps the runner's dispatch
// harmless for contracts nobody registered with the watchdog.
func TestExpiryNotifierWithoutSubscriptionsIsANoOp(t *testing.T) {
	newExpiryNotifier(&mockSubscriptionLister{}, quietLogger()).Notify(context.Background(), expiryAlert())
	// A store that cannot answer must not panic the pass.
	newExpiryNotifier(&mockSubscriptionLister{listErr: errors.New("db down")}, quietLogger()).Notify(context.Background(), expiryAlert())
	// A nil lister (store without the subscription surface) is inert.
	newExpiryNotifier(nil, quietLogger()).Notify(context.Background(), expiryAlert())
}

// TestContractSubscriptionsMapsEveryField guards the pgstore -> watchdog
// mirror-type conversion, where a renamed or dropped field would silently stop
// Slack/Discord/PagerDuty channels from firing.
func TestContractSubscriptionsMapsEveryField(t *testing.T) {
	lister := &mockSubscriptionLister{subs: []pgstore.AlertSubscription{
		{ID: "1", ContractID: "CDXQ", WebhookURL: "https://hooks.example/x", SeverityFilter: "Critical", ChannelType: "pagerduty", RoutingKey: "rk"},
		{ID: "2", ContractID: "CDXQ", WebhookURL: "https://discord.example/y", SeverityFilter: "Warning", ChannelType: "discord"},
	}}
	subs, err := (&contractSubscriptions{lister: lister}).ListByContract(context.Background(), "CDXQ")
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 2 {
		t.Fatalf("subs = %d, want 2", len(subs))
	}
	if subs[0].ID != "1" || subs[0].ContractID != "CDXQ" || subs[0].WebhookURL != "https://hooks.example/x" ||
		subs[0].SeverityFilter != "Critical" || subs[0].ChannelType != "pagerduty" || subs[0].RoutingKey != "rk" {
		t.Fatalf("first subscription lost fields in conversion: %+v", subs[0])
	}
	if subs[1].ChannelType != "discord" || subs[1].RoutingKey != "" {
		t.Fatalf("second subscription = %+v", subs[1])
	}

	errStore := &mockSubscriptionLister{listErr: errors.New("boom")}
	if _, err := (&contractSubscriptions{lister: errStore}).ListByContract(context.Background(), "CDXQ"); err == nil {
		t.Fatal("store errors must surface to the notifier")
	}
}
