package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
)

// IntegrationInfo describes one notification integration (issue #612): what it
// is, how to install it, and whether this deployment can deliver to it.
type IntegrationInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	InstallURL  string   `json:"install_url"`
	SetupSteps  []string `json:"setup_steps"`
	Configured  bool     `json:"configured"`
}

// integrationCatalog is the source of truth the Integrations page renders.
// `configured` is computed per request from the environment.
var integrationCatalog = []struct {
	info         IntegrationInfo
	requiredEnv  []string
	optionalEnv  []string
	testEndpoint func(env func(string) string) (string, []byte, string, error)
}{
	{
		info: IntegrationInfo{
			ID:          "slack",
			Name:        "Slack",
			Description: "Post watchdog alerts and use the /sorolens slash command inside a Slack workspace.",
			InstallURL:  "https://api.slack.com/apps?new_app=1",
			SetupSteps: []string{
				"Create a Slack app and install it to your workspace.",
				"Set SLACK_SIGNING_SECRET so slash commands are verified (POST /integrations/slack/commands).",
				"Create an incoming webhook for the target channel and set SLACK_WEBHOOK_URL.",
			},
		},
		requiredEnv: []string{"SLACK_WEBHOOK_URL"},
		optionalEnv: []string{"SLACK_SIGNING_SECRET"},
		testEndpoint: func(env func(string) string) (string, []byte, string, error) {
			hook := env("SLACK_WEBHOOK_URL")
			if hook == "" {
				return "", nil, "", errNotConfigured("SLACK_WEBHOOK_URL")
			}
			body, _ := json.Marshal(map[string]string{
				"text": "Sorolens test message: Slack integration is working. ✅",
			})
			return hook, body, "application/json", nil
		},
	},
	{
		info: IntegrationInfo{
			ID:          "discord",
			Name:        "Discord",
			Description: "Relay watchdog alerts to a Discord channel via the Sorolens Discord bot.",
			InstallURL:  "https://discord.com/developers/applications",
			SetupSteps: []string{
				"Create a Discord application and invite the bot to your server (see docs/discord-bot.md).",
				"Set DISCORD_BOT_TOKEN for the bot, or create a channel webhook and set DISCORD_WEBHOOK_URL.",
			},
		},
		requiredEnv: []string{"DISCORD_WEBHOOK_URL"},
		optionalEnv: []string{"DISCORD_BOT_TOKEN"},
		testEndpoint: func(env func(string) string) (string, []byte, string, error) {
			hook := env("DISCORD_WEBHOOK_URL")
			if hook == "" {
				return "", nil, "", errNotConfigured("DISCORD_WEBHOOK_URL")
			}
			body, _ := json.Marshal(map[string]string{
				"content": "Sorolens test message: Discord integration is working. ✅",
			})
			return hook, body, "application/json", nil
		},
	},
	{
		info: IntegrationInfo{
			ID:          "telegram",
			Name:        "Telegram",
			Description: "Deliver watchdog alerts to a Telegram chat through a bot.",
			InstallURL:  "https://t.me/BotFather",
			SetupSteps: []string{
				"Create a bot with @BotFather and copy the bot token.",
				"Add the bot to your chat and note the chat id.",
				"Set TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID.",
			},
		},
		requiredEnv: []string{"TELEGRAM_BOT_TOKEN", "TELEGRAM_CHAT_ID"},
		testEndpoint: func(env func(string) string) (string, []byte, string, error) {
			token, chat := env("TELEGRAM_BOT_TOKEN"), env("TELEGRAM_CHAT_ID")
			if token == "" || chat == "" {
				return "", nil, "", errNotConfigured("TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID")
			}
			form := url.Values{}
			form.Set("chat_id", chat)
			form.Set("text", "Sorolens test message: Telegram integration is working. ✅")
			endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
			return endpoint, []byte(form.Encode()), "application/x-www-form-urlencoded", nil
		},
	},
}

type notConfiguredError struct{ vars string }

func (e notConfiguredError) Error() string {
	return fmt.Sprintf("integration is not configured: set %s", e.vars)
}

func errNotConfigured(vars string) error { return notConfiguredError{vars: vars} }

// env returns the environment lookup for integration configuration. Tests
// override Handler.Getenv; production falls back to os.Getenv.
func (h *Handler) env(key string) string {
	if h.Getenv != nil {
		return h.Getenv(key)
	}
	return os.Getenv(key)
}

// ListIntegrations handles GET /api/v1/integrations: the catalog the web app's
// Integrations page renders, with per-integration configured status.
func (h *Handler) ListIntegrations(w http.ResponseWriter, r *http.Request) {
	integrations := make([]IntegrationInfo, 0, len(integrationCatalog))
	for _, entry := range integrationCatalog {
		info := entry.info
		info.Configured = allSet(h.env, entry.requiredEnv)
		integrations = append(integrations, info)
	}
	writeJSON(w, http.StatusOK, map[string]any{"integrations": integrations})
}

// TestIntegration handles POST /api/v1/integrations/{id}/test: it sends a real
// test message through the integration's configured channel, so users can
// verify their setup end to end.
func (h *Handler) TestIntegration(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	
	// Unknown IDs are rejected early so we never iterate the catalog for
	// an id that does not exist.
	var idMatch bool
	for _, entry := range integrationCatalog {
		if entry.info.ID == id {
			idMatch = true
			break
		}
	}
	if !idMatch {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("unknown integration %q (slack, discord, telegram)", id),
		})
		return
	}
	// Find the matching integration and check its environment together so we
	// never attempt to call out with partial configuration (e.g. a Telegram
	// token without a chat id).
	var endpoint string
	var payload []byte
	var contentType string
	for _, entry := range integrationCatalog {
		if entry.info.ID != id {
			continue
		}
		ep, body, ct, err2 := entry.testEndpoint(h.env)
		if err2 != nil {
			writeJSON(w, http.StatusConflict, map[string]any{
				"ok":    false,
				"error": err2.Error(),
			})
			return
		}
		endpoint, payload, contentType = ep, body, ct
		break
	}
	
	detail, err := h.postTestMessage(r.Context(), endpoint, payload, contentType)
	if err != nil {
		h.Logger.Warn("integration test send failed", "integration", id, "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("test message failed: %v", err),
		})
		return
	}
	
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "detail": detail})
}
	return endpoint, payload, contentType, nil
}

// postTestMessage delivers the payload and reads a bounded response so the
// connection can be reused and error bodies can be surfaced to the user.
func (h *Handler) postTestMessage(ctx context.Context, endpoint string, body []byte, contentType string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", contentType)
	client := h.IntegrationHTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("endpoint returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return fmt.Sprintf("test message delivered (endpoint returned %d)", resp.StatusCode), nil
}

func allSet(env func(string) string, keys []string) bool {
	for _, k := range keys {
		if env(k) == "" {
			return false
		}
	}
	return true
}

// integrationExists returns true when id is one of the catalog IDs. Tests reuse
// it to confirm the unknown-id path still returns 404.
func integrationExists(id string) bool {
	for _, entry := range integrationCatalog {
		if entry.info.ID == id {
			return true
		}
	}
	return false
}
