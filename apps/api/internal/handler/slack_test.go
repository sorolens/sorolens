package handler_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/router"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const slackSecret = "8f742231b10e8888abcd99yyyzzz85a5"

func slackSign(secret, ts, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("v0:" + ts + ":" + body))
	return "v0=" + hex.EncodeToString(mac.Sum(nil))
}

func newSlackServer(ms *store.MockStore, secret string) http.Handler {
	return router.New(&handler.Handler{
		Store:              ms,
		DB:                 &store.MockPinger{Healthy: true},
		Redis:              &store.MockPinger{Healthy: true},
		RedisClient:        &mockRedisClient{},
		Logger:             slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		SlackSigningSecret: secret,
	}, 1<<20)
}

func slackCommand(srv http.Handler, body, ts, sig string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/integrations/slack/commands", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Slack-Request-Timestamp", ts)
	req.Header.Set("X-Slack-Signature", sig)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

// TestVerifySlackSignatureKnownVector checks the verifier against the worked
// example in Slack's "Verifying requests from Slack" documentation.
func TestVerifySlackSignatureKnownVector(t *testing.T) {
	body := "token=xyzz0WbapA4vBCDEFasx0q6G&team_id=T1DC2JH3J&team_domain=testteamnow&channel_id=G8PSS9T3V&channel_name=foobar&user_id=U2CERLKJA&user_name=roadrunner&command=%2Fwebhook-collect&text=&response_url=https%3A%2F%2Fhooks.slack.com%2Fcommands%2FT1DC2JH3J%2F397700885554%2F96rGlfmibIGlgcZRskXaIFfN&trigger_id=398738663015.47445629121.803a0bc887a14d10d2c447fce8b6703c"
	const (
		ts  = "1531420618"
		sig = "v0=a2114d57b48eac39b9ad189dd8316235a7b4a8d21a10bd27519666489c69b503"
	)
	now := time.Unix(1531420618, 0)
	if err := handler.VerifySlackSignature(slackSecret, ts, sig, []byte(body), now); err != nil {
		t.Fatalf("known-good request rejected: %v", err)
	}
	if err := handler.VerifySlackSignature(slackSecret, ts, sig, []byte(body+"x"), now); err == nil {
		t.Fatal("tampered body accepted")
	}
	if err := handler.VerifySlackSignature(slackSecret, ts, sig, []byte(body), now.Add(6*time.Minute)); err == nil {
		t.Fatal("replayed (stale) request accepted")
	}
	if err := handler.VerifySlackSignature("", ts, sig, []byte(body), now); err == nil {
		t.Fatal("empty secret accepted")
	}
}

func TestSlackCommandEndpoint(t *testing.T) {
	ms := store.NewMockStore()
	check := time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC)
	_ = ms.UpsertMonitoredContract(context.Background(), store.MonitoredContract{
		ContractID: explorerContractA, Network: "testnet", Name: "vault", Status: "Degraded", LastCheck: &check,
	})
	_ = ms.InsertContractAlert(context.Background(), store.ContractAlert{
		ContractID: explorerContractA, Severity: "Critical", Message: "missed heartbeat", TxHash: "t1", Timestamp: check,
	})
	srv := newSlackServer(ms, slackSecret)
	now := strconv.FormatInt(time.Now().Unix(), 10)

	form := url.Values{"command": {"/sorolens"}, "text": {strings.ToLower(explorerContractA)}}.Encode()
	w := slackCommand(srv, form, now, slackSign(slackSecret, now, form))
	if w.Code != http.StatusOK {
		t.Fatalf("valid request: %d %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["response_type"] != "ephemeral" || resp["text"] != "vault is Degraded" {
		t.Fatalf("unexpected response: %v", resp)
	}
	if blocks, _ := resp["blocks"].([]any); len(blocks) != 2 {
		t.Fatalf("want status + alerts blocks, got %v", resp["blocks"])
	}

	// Unknown contract and help both answer politely.
	form = url.Values{"command": {"/sorolens"}, "text": {"CUNKNOWN"}}.Encode()
	if w := slackCommand(srv, form, now, slackSign(slackSecret, now, form)); !strings.Contains(w.Body.String(), "not monitored") {
		t.Fatalf("unknown contract: %s", w.Body.String())
	}
	form = url.Values{"command": {"/sorolens"}, "text": {""}}.Encode()
	if w := slackCommand(srv, form, now, slackSign(slackSecret, now, form)); !strings.Contains(w.Body.String(), "Usage") {
		t.Fatalf("help: %s", w.Body.String())
	}

	// Bad signature, wrong secret, and a stale timestamp are rejected.
	stale := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	for name, req := range map[string][2]string{
		"bad signature": {now, "v0=deadbeef"},
		"wrong secret":  {now, slackSign("other", now, form)},
		"stale":         {stale, slackSign(slackSecret, stale, form)},
		"missing":       {"", ""},
	} {
		if w := slackCommand(srv, form, req[0], req[1]); w.Code != http.StatusUnauthorized {
			t.Errorf("%s: want 401, got %d", name, w.Code)
		}
	}

	// Without a signing secret the endpoint is disabled.
	if w := slackCommand(newSlackServer(ms, ""), form, now, slackSign("", now, form)); w.Code != http.StatusNotFound {
		t.Errorf("unconfigured: want 404, got %d", w.Code)
	}
}
