package watchdog

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/sorolens/sorolens/services/indexer/internal/webhooksig"
)

// recordingSubStore is a static AlertSubscriptionStore for tests.
type recordingSubStore struct {
	subs []AlertSubscription
}

func (s *recordingSubStore) ListByContract(_ context.Context, contractID string) ([]AlertSubscription, error) {
	var out []AlertSubscription
	for _, sub := range s.subs {
		if sub.ContractID == contractID {
			out = append(out, sub)
		}
	}
	return out, nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type recordedDelivery struct {
	signature string
	timestamp string
	body      []byte
	ctype     string
}

// deliveryRecorder collects each request the test server receives.
type deliveryRecorder struct {
	mu         sync.Mutex
	deliveries []recordedDelivery
	status     int
}

func (r *deliveryRecorder) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		r.mu.Lock()
		r.deliveries = append(r.deliveries, recordedDelivery{
			signature: req.Header.Get(webhooksig.SignatureHeader),
			timestamp: req.Header.Get(webhooksig.TimestampHeader),
			body:      body,
			ctype:     req.Header.Get("Content-Type"),
		})
		status := r.status
		r.mu.Unlock()
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
	}
}

func (r *deliveryRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.deliveries)
}

func (r *deliveryRecorder) all() []recordedDelivery {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]recordedDelivery(nil), r.deliveries...)
}

// waitForCount blocks until the recorder has seen want deliveries, or fails.
func (r *deliveryRecorder) waitForCount(t *testing.T, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if r.count() >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d deliveries, saw %d", want, r.count())
}

func testAlert() Alert {
	return Alert{
		ContractID: "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Severity:   "Critical",
		Message:    "cpu spike",
		TxHash:     "deadbeef",
		Timestamp:  time.Now().UTC(),
	}
}

func TestDispatchAlerts_SignsDelivery(t *testing.T) {
	secret, err := webhooksig.GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}

	rec := &deliveryRecorder{}
	srv := httptest.NewServer(rec.handler())
	defer srv.Close()

	store := &recordingSubStore{subs: []AlertSubscription{{
		ID:             "sub_1",
		ContractID:     testAlert().ContractID,
		WebhookURL:     srv.URL,
		SeverityFilter: "Critical",
		SigningSecret:  secret,
	}}}

	DispatchAlerts(context.Background(), testAlert(), store, discardLogger())
	rec.waitForCount(t, 1)

	d := rec.all()[0]
	if d.ctype != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", d.ctype)
	}
	if d.timestamp == "" {
		t.Error("X-Sorolens-Timestamp header is missing")
	}
	if d.signature == "" {
		t.Fatal("X-Sorolens-Signature header is missing")
	}
	// The receiver's algorithm must accept what we sent.
	if err := webhooksig.Verify(secret, d.signature, d.timestamp, d.body, time.Now(), 0); err != nil {
		t.Fatalf("delivered signature failed verification: %v", err)
	}
	if len(d.body) == 0 {
		t.Error("delivery body is empty")
	}
}

func TestDispatchAlerts_NonCriticalIsIgnored(t *testing.T) {
	rec := &deliveryRecorder{}
	srv := httptest.NewServer(rec.handler())
	defer srv.Close()

	secret, _ := webhooksig.GenerateSecret()
	store := &recordingSubStore{subs: []AlertSubscription{{
		ID: "sub_1", ContractID: testAlert().ContractID,
		WebhookURL: srv.URL, SeverityFilter: "Critical", SigningSecret: secret,
	}}}

	alert := testAlert()
	alert.Severity = "Info"
	DispatchAlerts(context.Background(), alert, store, discardLogger())

	time.Sleep(150 * time.Millisecond)
	if rec.count() != 0 {
		t.Errorf("non-critical alert produced %d deliveries", rec.count())
	}
}

func TestDispatchAlerts_SkipsSubscriptionWithoutSecret(t *testing.T) {
	rec := &deliveryRecorder{}
	srv := httptest.NewServer(rec.handler())
	defer srv.Close()

	store := &recordingSubStore{subs: []AlertSubscription{{
		ID: "sub_1", ContractID: testAlert().ContractID,
		WebhookURL: srv.URL, SeverityFilter: "Critical", SigningSecret: "",
	}}}

	DispatchAlerts(context.Background(), testAlert(), store, discardLogger())

	time.Sleep(150 * time.Millisecond)
	if rec.count() != 0 {
		t.Errorf("a subscription without a signing secret must not be delivered to, got %d", rec.count())
	}
}

func TestDispatchAlerts_SeverityFilterSkips(t *testing.T) {
	rec := &deliveryRecorder{}
	srv := httptest.NewServer(rec.handler())
	defer srv.Close()

	secret, _ := webhooksig.GenerateSecret()
	store := &recordingSubStore{subs: []AlertSubscription{{
		ID: "sub_1", ContractID: testAlert().ContractID,
		WebhookURL: srv.URL, SeverityFilter: "Info", SigningSecret: secret,
	}}}

	DispatchAlerts(context.Background(), testAlert(), store, discardLogger())

	time.Sleep(150 * time.Millisecond)
	if rec.count() != 0 {
		t.Errorf("severity filter should have skipped the delivery, got %d", rec.count())
	}
}

func TestDispatchAlerts_RetriesOnceOn5xx(t *testing.T) {
	secret, _ := webhooksig.GenerateSecret()
	rec := &deliveryRecorder{status: http.StatusInternalServerError}
	srv := httptest.NewServer(rec.handler())
	defer srv.Close()

	store := &recordingSubStore{subs: []AlertSubscription{{
		ID: "sub_1", ContractID: testAlert().ContractID,
		WebhookURL: srv.URL, SeverityFilter: "Critical", SigningSecret: secret,
	}}}

	DispatchAlerts(context.Background(), testAlert(), store, discardLogger())
	rec.waitForCount(t, 2)

	// Give any erroneous third attempt time to arrive.
	time.Sleep(100 * time.Millisecond)
	if got := rec.count(); got != 2 {
		t.Fatalf("want exactly one retry (2 attempts), got %d", got)
	}
	// Both attempts must verify, i.e. the retry re-sent the same signed bytes.
	for i, d := range rec.all() {
		if err := webhooksig.Verify(secret, d.signature, d.timestamp, d.body, time.Now(), 0); err != nil {
			t.Errorf("attempt %d signature invalid: %v", i+1, err)
		}
	}
}

func TestDispatchAlerts_DoesNotRetryOn4xx(t *testing.T) {
	secret, _ := webhooksig.GenerateSecret()
	rec := &deliveryRecorder{status: http.StatusBadRequest}
	srv := httptest.NewServer(rec.handler())
	defer srv.Close()

	store := &recordingSubStore{subs: []AlertSubscription{{
		ID: "sub_1", ContractID: testAlert().ContractID,
		WebhookURL: srv.URL, SeverityFilter: "Critical", SigningSecret: secret,
	}}}

	DispatchAlerts(context.Background(), testAlert(), store, discardLogger())
	rec.waitForCount(t, 1)

	time.Sleep(100 * time.Millisecond)
	if got := rec.count(); got != 1 {
		t.Errorf("a 4xx must not be retried, got %d attempts", got)
	}
}

func TestDispatchAlerts_NoSubscriptionsDoesNothing(t *testing.T) {
	store := &recordingSubStore{}
	// Must not panic and must return promptly.
	DispatchAlerts(context.Background(), testAlert(), store, discardLogger())
}
