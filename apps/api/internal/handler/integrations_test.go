package handler_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/config"
	"github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/router"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// newIntegrationsHandler builds a router whose handler reads integration env
// from env and delivers test messages through the given client.
func newIntegrationsHandler(env map[string]string, client *http.Client) http.Handler {
	h := &handler.Handler{
		Store:  store.NewMockStore(),
		DB:     &store.MockPinger{Healthy: true},
		Redis:  &store.MockPinger{Healthy: true},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Getenv: func(k string) string { return env[k] },
		Client: client,
	}
	return router.New(h, config.DefaultRequestMaxBodyBytes)
}

func TestListIntegrations(t *testing.T) {
	srv := newIntegrationsHandler(map[string]string{
		"SLACK_WEBHOOK_URL": "https://hooks.slack.test/x",
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/integrations", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var body struct {
		Integrations []struct {
			ID         string   `json:"id"`
			Name       string   `json:"name"`
			Configured bool     `json:"configured"`
			SetupSteps []string `json:"setup_steps"`
			InstallURL string   `json:"install_url"`
		} `json:"integrations"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Integrations) != 3 {
		t.Fatalf("want 3 integrations, got %d", len(body.Integrations))
	}
	byID := map[string]struct {
		Configured bool
		Steps      int
	}{}
	for _, in := range body.Integrations {
		byID[in.ID] = struct {
			Configured bool
			Steps      int
		}{in.Configured, len(in.SetupSteps)}
		if in.Name == "" {
			t.Errorf("integration %s has no name", in.ID)
		}
		if in.InstallURL == "" {
			t.Errorf("integration %s has no install_url", in.ID)
		}
	}
	slack, ok := byID["slack"]
	if !ok {
		t.Fatal("slack missing from catalog")
	}
	if !slack.Configured {
		t.Error("slack should be configured when SLACK_WEBHOOK_URL is set")
	}
	if slack.Steps == 0 {
		t.Error("slack needs setup steps")
	}
	if discord := byID["discord"]; discord.Configured {
		t.Error("discord must not report configured without env")
	}
	if telegram := byID["telegram"]; telegram.Configured {
		t.Error("telegram must not report configured without env")
	}
}

func TestTestIntegration(t *testing.T) {
	cases := []struct {
		name       string
		id         string
		env        map[string]string
		wantStatus int
	}{
		{"slack configured", "slack", map[string]string{"SLACK_WEBHOOK_URL": "SET"}, http.StatusOK},
		{"slack not configured", "slack", map[string]string{}, http.StatusConflict},
		{"discord configured", "discord", map[string]string{"DISCORD_WEBHOOK_URL": "SET"}, http.StatusOK},
		{"telegram not configured", "telegram", map[string]string{"TELEGRAM_BOT_TOKEN": "t"}, http.StatusConflict},
		{"unknown integration", "pagerduty", map[string]string{}, http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if len(body) == 0 {
					t.Error("test message body is empty")
				}
				if !strings.Contains(string(body), "Sorolens test message") {
					t.Errorf("body does not identify itself as a test: %s", body)
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer ts.Close()

			env := map[string]string{}
			for k := range tc.env {
				env[k] = ts.URL // point configured webhooks at the test server
			}
			srv := newIntegrationsHandler(env, ts.Client())

			req := httptest.NewRequest(http.MethodPost, "/api/v1/integrations/"+tc.id+"/test", nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)
			if w.Code != tc.wantStatus {
				t.Fatalf("want %d, got %d: %s", tc.wantStatus, w.Code, w.Body.String())
			}

			var body struct {
				OK     bool   `json:"ok"`
				Error  string `json:"error"`
				Detail string `json:"detail"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			switch tc.wantStatus {
			case http.StatusOK:
				if !body.OK || body.Detail == "" {
					t.Errorf("want ok=true with detail, got %+v", body)
				}
			case http.StatusConflict, http.StatusNotFound:
				if body.OK || body.Error == "" {
					t.Errorf("want ok=false with error, got %+v", body)
				}
			}
		})
	}
}

func TestTestIntegrationEndpointFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "channel_not_found", http.StatusNotFound)
	}))
	defer ts.Close()
	srv := newIntegrationsHandler(map[string]string{"DISCORD_WEBHOOK_URL": ts.URL}, ts.Client())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/integrations/discord/test", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)			if w.Code != http.StatusBadGateway {
				t.Fatalf("want 502 when the remote rejects the message, got %d", w.Code)
			}

			// Smoke test: the handler compiles and the two catalog endpoints respond.
			func TestIntegrationCatalogSmoke(t *testing.T) {
			srv := newIntegrationsHandler(map[string]string{
				"SLACK_WEBHOOK_URL":   "https://hooks.slack.ok/s",
				"DISCORD_WEBHOOK_URL": "https://discord.ok/d",
				"TELEGRAM_BOT_TOKEN":  "tok",
				"TELEGRAM_CHAT_ID":    "42",
	}, nil)

	// List endpoint
	req := httptest.NewRequest(http.MethodGet, "/api/v1/integrations", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListIntegrations: want 200, got %d - %s", w.Code, w.Body.String())
	}

	// Smoke parse
	var bodyList struct {
		Integrations []interface{} `json:"integrations"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &bodyList); err != nil {
		t.Fatalf("could not unmarshal /integrations: %v", err)
	}
	if len(bodyList.Integrations) != 3 {
		t.Fatalf("want 3 integrations, got %d", len(bodyList.Integrations))
	}
	
	// Smoke test-send endpoint with configured integration
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/integrations/slack/test", nil)
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("TestIntegration(slack): want 200, got %d - %s", w2.Code, w2.Body.String())
	}
	var body2 struct {
		OK     bool   `json:"ok"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &body2); err != nil {
		t.Fatal(err)
	}
	if !body2.OK || body2.Detail == "" {
		t.Fatalf("want ok=true with detail for slack integration, got %+v", body2)
	}
}		Error string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.OK || !strings.Contains(body.Error, "404") {
		t.Errorf("want ok=false mentioning the remote status, got %+v", body)
	}
}

func TestTestIntegrationTelegramForm(t *testing.T) {
	// Telegram's endpoint is built from the bot token, so point the handler's
	// HTTP client at a fake and confirm partial configuration is a 409 rather
	// than an attempted call to the real api.telegram.org.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("must not call out when telegram is only partially configured: %s", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	srv := newIntegrationsHandler(map[string]string{"TELEGRAM_BOT_TOKEN": "TOKEN"}, ts.Client())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/integrations/telegram/test", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409 with partial telegram config, got %d", w.Code)
	}
}
