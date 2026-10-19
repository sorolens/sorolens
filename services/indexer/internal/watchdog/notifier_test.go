package watchdog

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sorolens/sorolens/services/indexer/internal/webhooksig"
)

const (
	notifyContract = "CABQGAYDAMBQGAYDAMBQGAYDAMBQGAYDAMBQGAYDAMBQGAYDAMBQGCK3"
	notifyTx       = "8c1f0e4b8a7d2c9e6b5a4f3e2d1c0b9a8f7e6d5c4b3a29181716151413121110"
)

func criticalAlert() Alert {
	return Alert{
		ContractID: notifyContract,
		Severity:   "Critical",
		Message:    "health check missed 3 intervals",
		Ledger:     123456,
		TxHash:     notifyTx,
		Timestamp:  time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC),
	}
}

// decode renders a formatted notification back into a generic map.
func decode(t *testing.T, sub AlertSubscription) (string, map[string]any) {
	t.Helper()
	url, body, err := FormatNotification(sub, criticalAlert())
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	return url, out
}

// dig walks a decoded JSON value by map keys and slice indexes.
func dig(t *testing.T, v any, path ...any) any {
	t.Helper()
	for _, p := range path {
		switch k := p.(type) {
		case string:
			m, ok := v.(map[string]any)
			if !ok {
				t.Fatalf("path %v: want object at %q, got %T", path, k, v)
			}
			v = m[k]
		case int:
			s, ok := v.([]any)
			if !ok || k >= len(s) {
				t.Fatalf("path %v: want array with index %d, got %T", path, k, v)
			}
			v = s[k]
		}
	}
	return v
}

func TestFormatWebhook(t *testing.T) {
	url, p := decode(t, AlertSubscription{ID: "s1", WebhookURL: "https://example.com/hook"})
	if url != "https://example.com/hook" {
		t.Fatalf("url: %s", url)
	}
	want := map[string]any{
		"contract_id":  notifyContract,
		"severity":     "Critical",
		"message":      "health check missed 3 intervals",
		"timestamp":    "2026-09-25T10:00:00Z",
		"explorer_url": "https://sorobanexplorer.com/transaction/" + notifyTx,
	}
	for k, v := range want {
		if p[k] != v {
			t.Errorf("%s: want %v, got %v", k, v, p[k])
		}
	}
}

func TestFormatSlackBlockKit(t *testing.T) {
	url, p := decode(t, AlertSubscription{ID: "s1", ChannelType: ChannelSlack, WebhookURL: "https://hooks.slack.com/services/T/B/X"})
	if url != "https://hooks.slack.com/services/T/B/X" {
		t.Fatalf("url: %s", url)
	}
	if text, _ := p["text"].(string); !strings.Contains(text, ":rotating_light:") || !strings.Contains(text, notifyContract) {
		t.Errorf("fallback text: %q", text)
	}
	if got := dig(t, p, "blocks", 0, "type"); got != "header" {
		t.Errorf("first block: %v", got)
	}
	if got := dig(t, p, "blocks", 0, "text", "text"); got != "Critical alert" {
		t.Errorf("header text: %v", got)
	}
	if got := dig(t, p, "blocks", 1, "fields", 0, "text"); got != "*Contract*\n`"+notifyContract+"`" {
		t.Errorf("contract field: %v", got)
	}
	if got := dig(t, p, "blocks", 2, "text", "text"); got != "health check missed 3 intervals" {
		t.Errorf("message: %v", got)
	}
	if got := dig(t, p, "blocks", 4, "elements", 0, "url"); got != "https://sorobanexplorer.com/transaction/"+notifyTx {
		t.Errorf("button url: %v", got)
	}
}

func TestFormatDiscordEmbed(t *testing.T) {
	_, p := decode(t, AlertSubscription{ID: "s1", ChannelType: ChannelDiscord, WebhookURL: "https://discord.com/api/webhooks/1/abc"})
	if p["username"] != "Sorolens" {
		t.Errorf("username: %v", p["username"])
	}
	embed := dig(t, p, "embeds", 0)
	if got := dig(t, embed, "title"); got != "Critical alert" {
		t.Errorf("title: %v", got)
	}
	if got := dig(t, embed, "color"); got != float64(discordColorCritical) {
		t.Errorf("color: %v", got)
	}
	if got := dig(t, embed, "timestamp"); got != "2026-09-25T10:00:00Z" {
		t.Errorf("timestamp: %v", got)
	}
	if got := dig(t, embed, "fields", 0, "value"); got != "`"+notifyContract+"`" {
		t.Errorf("contract field: %v", got)
	}
	if got := dig(t, embed, "fields", 2, "value"); got != "123456" {
		t.Errorf("ledger field: %v", got)
	}
}

func TestFormatPagerDutyEventsV2(t *testing.T) {
	url, p := decode(t, AlertSubscription{ID: "s1", ChannelType: ChannelPagerDuty, RoutingKey: "R0UT1NGKEY"})
	if url != PagerDutyEventsURL {
		t.Fatalf("url: want default events endpoint, got %s", url)
	}
	if p["routing_key"] != "R0UT1NGKEY" || p["event_action"] != "trigger" {
		t.Errorf("envelope: %v", p)
	}
	if p["dedup_key"] != "sorolens:"+notifyContract+":"+notifyTx {
		t.Errorf("dedup_key: %v", p["dedup_key"])
	}
	if got := dig(t, p, "payload", "severity"); got != "critical" {
		t.Errorf("severity: %v", got)
	}
	if got := dig(t, p, "payload", "source"); got != notifyContract {
		t.Errorf("source: %v", got)
	}
	if got := dig(t, p, "payload", "summary"); got != "[Critical] "+notifyContract+": health check missed 3 intervals" {
		t.Errorf("summary: %v", got)
	}
	if got := dig(t, p, "links", 0, "href"); got != "https://sorobanexplorer.com/transaction/"+notifyTx {
		t.Errorf("link: %v", got)
	}
}

func TestPagerDutySeverityMapping(t *testing.T) {
	for in, want := range map[string]string{"Critical": "critical", "Warning": "warning", "Info": "info"} {
		if got := pagerDutySeverity(in); got != want {
			t.Errorf("%s: want %s, got %s", in, want, got)
		}
	}
}

func TestFormatNotificationErrors(t *testing.T) {
	cases := map[string]AlertSubscription{
		"pagerduty without key": {ID: "s", ChannelType: ChannelPagerDuty},
		"unknown channel":       {ID: "s", ChannelType: "carrier-pigeon", WebhookURL: "https://x"},
		"webhook without url":   {ID: "s", ChannelType: ChannelWebhook},
	}
	for name, sub := range cases {
		if _, _, err := FormatNotification(sub, criticalAlert()); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("héllo wörld", 5); got != "héll…" {
		t.Fatalf("got %q", got)
	}
	if got := truncate("short", 10); got != "short" {
		t.Fatalf("got %q", got)
	}
}

type fakeSubStore struct{ subs []AlertSubscription }

func (f fakeSubStore) ListByContract(_ context.Context, _ string) ([]AlertSubscription, error) {
	return f.subs, nil
}

// TestDispatchDeliversEachChannel sends a Critical alert to one subscription
// per channel and checks each endpoint receives its own format, retrying a
// 5xx once with the full body.
func TestDispatchDeliversEachChannel(t *testing.T) {
	var (
		mu       sync.Mutex
		received = map[string][]map[string]any{}
		failed   bool
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var p map[string]any
		if err := json.Unmarshal(b, &p); err != nil {
			t.Errorf("%s: body is not JSON (%q)", r.URL.Path, b)
		}
		mu.Lock()
		received[r.URL.Path] = append(received[r.URL.Path], p)
		// The first PagerDuty delivery fails with a 5xx to exercise retry.
		fail := r.URL.Path == "/pagerduty" && !failed
		if fail {
			failed = true
		}
		mu.Unlock()
		if fail {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	subs := fakeSubStore{subs: []AlertSubscription{
		{ID: "w", ChannelType: ChannelWebhook, WebhookURL: srv.URL + "/webhook", SeverityFilter: "Critical", SigningSecret: exampleSigningSecret},
		{ID: "s", ChannelType: ChannelSlack, WebhookURL: srv.URL + "/slack", SeverityFilter: "Critical", SigningSecret: exampleSigningSecret},
		{ID: "d", ChannelType: ChannelDiscord, WebhookURL: srv.URL + "/discord", SeverityFilter: "Critical", SigningSecret: exampleSigningSecret},
		{ID: "p", ChannelType: ChannelPagerDuty, WebhookURL: srv.URL + "/pagerduty", RoutingKey: "k", SeverityFilter: "Critical", SigningSecret: exampleSigningSecret},
	}}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 4}))
	DispatchAlerts(context.Background(), criticalAlert(), subs, logger)

	deadline := time.Now().Add(3 * time.Second)
	for {
		mu.Lock()
		done := len(received["/webhook"]) == 1 && len(received["/slack"]) == 1 &&
			len(received["/discord"]) == 1 && len(received["/pagerduty"]) == 2
		mu.Unlock()
		if done {
			break
		}
		if time.Now().After(deadline) {
			mu.Lock()
			defer mu.Unlock()
			t.Fatalf("deliveries incomplete: %v", received)
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if received["/webhook"][0]["severity"] != "Critical" {
		t.Error("webhook payload")
	}
	if _, ok := received["/slack"][0]["blocks"]; !ok {
		t.Error("slack payload must carry blocks")
	}
	if _, ok := received["/discord"][0]["embeds"]; !ok {
		t.Error("discord payload must carry embeds")
	}
	for i, p := range received["/pagerduty"] {
		if p["routing_key"] != "k" {
			t.Errorf("pagerduty attempt %d lost its body: %v", i+1, p)
		}
	}
}

func TestDispatchSkipsNonCritical(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer srv.Close()
	a := criticalAlert()
	a.Severity = "Warning"
	DispatchAlerts(context.Background(), a, fakeSubStore{subs: []AlertSubscription{{ID: "w", WebhookURL: srv.URL, SeverityFilter: "Warning", SigningSecret: exampleSigningSecret}}},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	time.Sleep(50 * time.Millisecond)
	if called {
		t.Fatal("non-critical alerts must not be dispatched")
	}
}

// ---- signing ---------------------------------------------------------------

// exampleSigningSecret is the secret from the worked example in
// docs/webhooks.md. Keeping the tests on the published value means a receiver
// implemented from the docs and the sender here cannot drift apart.
const exampleSigningSecret = "whsec_example_0123456789abcdef0123456789abcdef"

// TestSignHeadersMatchPublishedVector pins the emitted headers to the test
// vector in docs/webhooks.md.
func TestSignHeadersMatchPublishedVector(t *testing.T) {
	body := []byte(`{"event":"watchdog.alert","severity":"Critical"}`)
	headers := signHeaders(exampleSigningSecret, body, time.Unix(1700000000, 0))

	want := "t=1700000000,v1=97dc0f80c8e0c9be056f34e48e360411160395a1b3e3a7978092d869315c4615"
	if got := headers[webhooksig.SignatureHeader]; got != want {
		t.Fatalf("signature header:\n got %q\nwant %q", got, want)
	}
	if got := headers[webhooksig.TimestampHeader]; got != "1700000000" {
		t.Fatalf("timestamp header: got %q", got)
	}
}

// TestSignHeadersSignExactBody is the property that matters to a receiver: the
// MAC covers the bytes on the wire, so re-serialising the JSON invalidates it.
func TestSignHeadersSignExactBody(t *testing.T) {
	body := []byte(`{"a":1}`)
	headers := signHeaders(exampleSigningSecret, body, time.Now())

	ts, _ := strconv.ParseInt(headers[webhooksig.TimestampHeader], 10, 64)
	for _, tampered := range [][]byte{[]byte(`{"a": 1}`), []byte(`{"a":2}`), nil} {
		if err := webhooksig.Verify(exampleSigningSecret, headers[webhooksig.SignatureHeader],
			headers[webhooksig.TimestampHeader], tampered, time.Unix(ts, 0), 0); err == nil {
			t.Errorf("signature verified over a body it did not sign: %q", tampered)
		}
	}
	if err := webhooksig.Verify(exampleSigningSecret, headers[webhooksig.SignatureHeader],
		headers[webhooksig.TimestampHeader], body, time.Unix(ts, 0), 0); err != nil {
		t.Fatalf("receiver rejected a genuine delivery: %v", err)
	}
}

// TestDispatchSignsDeliveries is the receiver-side acceptance test: it takes
// the requests the notifier actually sends, runs webhooksig.Verify on them, and
// checks the invariants docs/webhooks.md promises — every delivery signed, the
// 5xx retry reusing the same signature, and no unsigned payload ever sent.
func TestDispatchSignsDeliveries(t *testing.T) {
	type delivery struct {
		body      []byte
		signature string
		timestamp string
	}
	var (
		mu       sync.Mutex
		received = map[string][]delivery{}
		retried  bool
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		received[r.URL.Path] = append(received[r.URL.Path], delivery{
			body:      body,
			signature: r.Header.Get(webhooksig.SignatureHeader),
			timestamp: r.Header.Get(webhooksig.TimestampHeader),
		})
		retry := r.URL.Path == "/retry" && !retried
		if retry {
			retried = true
		}
		mu.Unlock()
		if retry {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	subs := fakeSubStore{subs: []AlertSubscription{
		{ID: "signed", WebhookURL: srv.URL + "/signed", SeverityFilter: "Critical", SigningSecret: exampleSigningSecret},
		{ID: "retry", WebhookURL: srv.URL + "/retry", SeverityFilter: "Critical", SigningSecret: exampleSigningSecret},
		{ID: "unsigned", WebhookURL: srv.URL + "/unsigned", SeverityFilter: "Critical"},
	}}
	DispatchAlerts(context.Background(), criticalAlert(), subs, slog.New(slog.NewTextHandler(io.Discard, nil)))

	deadline := time.Now().Add(3 * time.Second)
	for {
		mu.Lock()
		done := len(received["/signed"]) == 1 && len(received["/retry"]) == 2
		mu.Unlock()
		if done {
			break
		}
		if time.Now().After(deadline) {
			mu.Lock()
			defer mu.Unlock()
			t.Fatalf("deliveries incomplete: %+v", received)
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()

	if got := len(received["/unsigned"]); got != 0 {
		t.Fatalf("a subscription without a signing secret must not be delivered to, got %d deliveries", got)
	}

	for _, path := range []string{"/signed", "/retry"} {
		for i, d := range received[path] {
			if d.signature == "" || d.timestamp == "" {
				t.Fatalf("%s delivery %d is missing signature headers: %+v", path, i+1, d)
			}
			if err := webhooksig.Verify(exampleSigningSecret, d.signature, d.timestamp, d.body, time.Now(), 0); err != nil {
				t.Fatalf("%s delivery %d does not verify: %v", path, i+1, err)
			}
		}
	}

	// docs/webhooks.md: "A 5xx response is retried once with the same signed
	// body and timestamp, so the retry verifies with the same signature."
	retry := received["/retry"]
	if retry[0].signature != retry[1].signature || retry[0].timestamp != retry[1].timestamp {
		t.Fatalf("retry changed the signature: %q/%q vs %q/%q",
			retry[0].signature, retry[0].timestamp, retry[1].signature, retry[1].timestamp)
	}
}
