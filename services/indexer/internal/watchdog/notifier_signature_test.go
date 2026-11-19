package watchdog

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sorolens/sorolens/services/indexer/internal/webhooksig"
)

// TestDispatchSignsDeliveries pins the wire contract: a produced request
// carries both signing headers and verifies with webhooksig.Verify. Tampering,
// a different secret and a stale timestamp are rejected with the documented
// errors.
func TestDispatchSignsDeliveries(t *testing.T) {
	const secret = "whsec_dispatch_test_0123456789abcdef"

	var (
		mu        sync.Mutex
		body      []byte
		signature string
		timestamp string
		got       = make(chan struct{}, 1)
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		body = b
		signature = r.Header.Get(webhooksig.SignatureHeader)
		timestamp = r.Header.Get(webhooksig.TimestampHeader)
		mu.Unlock()
		select {
		case got <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	DispatchAlerts(context.Background(), criticalAlert(), fakeSubStore{subs: []AlertSubscription{{
		ID:             "w",
		ChannelType:    ChannelWebhook,
		WebhookURL:     srv.URL,
		SeverityFilter: "Critical",
		SigningSecret:  secret,
	}}}, logger)

	select {
	case <-got:
	case <-time.After(3 * time.Second):
		t.Fatal("no delivery arrived")
	}

	mu.Lock()
	defer mu.Unlock()
	if signature == "" || timestamp == "" {
		t.Fatalf("missing signing headers: signature=%q timestamp=%q", signature, timestamp)
	}
	signedAt, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		t.Fatalf("timestamp header %q: %v", timestamp, err)
	}
	now := time.Unix(signedAt, 0)

	if err := webhooksig.Verify(secret, signature, timestamp, body, now, 0); err != nil {
		t.Fatalf("delivered request failed verification: %v", err)
	}
	if err := webhooksig.Verify(secret, signature, timestamp, []byte(`{"severity":"Info"}`), now, 0); !errors.Is(err, webhooksig.ErrSignatureMismatch) {
		t.Fatalf("tampered body: want ErrSignatureMismatch, got %v", err)
	}
	if err := webhooksig.Verify("whsec_other", signature, timestamp, body, now, 0); !errors.Is(err, webhooksig.ErrSignatureMismatch) {
		t.Fatalf("wrong secret: want ErrSignatureMismatch, got %v", err)
	}
	stale := now.Add(webhooksig.DefaultTolerance + time.Second)
	if err := webhooksig.Verify(secret, signature, timestamp, body, stale, 0); !errors.Is(err, webhooksig.ErrTimestampOutsideWindow) {
		t.Fatalf("stale timestamp: want ErrTimestampOutsideWindow, got %v", err)
	}
}

// TestDispatchRefusesUnsignedSubscriptions makes the no-secret rule
// observable: the endpoint is never contacted, so an unsigned payload cannot
// leave the indexer.
func TestDispatchRefusesUnsignedSubscriptions(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
	}))
	defer srv.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	DispatchAlerts(context.Background(), criticalAlert(), fakeSubStore{subs: []AlertSubscription{{
		ID:             "w",
		ChannelType:    ChannelWebhook,
		WebhookURL:     srv.URL,
		SeverityFilter: "Critical",
	}}}, logger)

	time.Sleep(100 * time.Millisecond)
	if n := hits.Load(); n != 0 {
		t.Fatalf("unsigned subscription received %d delivery(ies)", n)
	}
}
