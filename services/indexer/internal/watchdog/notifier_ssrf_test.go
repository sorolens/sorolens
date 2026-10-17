package watchdog

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

)

// TestMain swaps in a plain client: the other tests deliver to httptest
// servers, which listen on loopback and are refused by the production client.
func TestMain(m *testing.M) {
	httpClient = &http.Client{Timeout: 10 * time.Second}
	os.Exit(m.Run())
}

// TestDeliverRefusesInternalDestinations uses the production client
// configuration and checks that a subscription pointing at an internal address
// (including via a redirect from a reachable host) never gets a request.
func TestDeliverRefusesInternalDestinations(t *testing.T) {
	orig := httpClient
	httpClient = newGuardedClient(2*time.Second, nil, nil, isBlockedAddr)
	t.Cleanup(func() { httpClient = orig })

	var hits atomic.Int32
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = io.WriteString(w, "secret")
	}))
	defer internal.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for name, url := range map[string]string{
		"loopback":         internal.URL,
		"metadata":         "http://169.254.169.254/latest/meta-data/",
		"private":          "http://10.0.0.1/hook",
		"ipv6 loopback":    "http://[::1]:9/hook",
		"ipv4-mapped ipv6": "http://[::ffff:127.0.0.1]:9/hook",
	} {
		t.Run(name, func(t *testing.T) {
			sub := AlertSubscription{ID: "s", ChannelType: ChannelWebhook, WebhookURL: url, SeverityFilter: "Critical"}
			deliver(context.Background(), sub, criticalAlert(), logger)
		})
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("internal server received %d request(s)", n)
	}
}
