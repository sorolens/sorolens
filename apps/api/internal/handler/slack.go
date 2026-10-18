package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const (
	// slackMaxSkew rejects requests whose timestamp is further than this
	// from now, so a captured request cannot be replayed later.
	slackMaxSkew = 5 * time.Minute
	// slackMaxBody bounds the form body read before verification.
	slackMaxBody = 64 << 10
)

// VerifySlackSignature checks a Slack request signature (v0 scheme): the
// X-Slack-Signature header must equal "v0=" + hex(HMAC-SHA256(secret,
// "v0:" + timestamp + ":" + body)), and the X-Slack-Request-Timestamp must be
// within five minutes of now.
func VerifySlackSignature(secret, timestamp, signature string, body []byte, now time.Time) error {
	if secret == "" {
		return errors.New("slack signing secret not configured")
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return errors.New("invalid timestamp")
	}
	if d := now.Sub(time.Unix(ts, 0)); d > slackMaxSkew || d < -slackMaxSkew {
		return errors.New("stale timestamp")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("v0:" + timestamp + ":"))
	mac.Write(body)
	want := "v0=" + hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(want), []byte(signature)) {
		return errors.New("signature mismatch")
	}
	return nil
}

// SlackCommand handles POST /integrations/slack/commands, the Slack slash
// command endpoint (e.g. `/sorolens C...`). Every request is verified with
// the Slack signing secret before the form is parsed. It replies with an
// ephemeral Block Kit message showing the contract's watchdog status and its
// latest alerts.
func (h *Handler) SlackCommand(w http.ResponseWriter, r *http.Request) {
	if h.SlackSigningSecret == "" {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "slack integration is not configured")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, slackMaxBody+1))
	if err != nil || len(body) > slackMaxBody {
		writeError(w, r, http.StatusBadRequest, CodeInvalidInput, "invalid request body")
		return
	}
	if err := VerifySlackSignature(h.SlackSigningSecret,
		r.Header.Get("X-Slack-Request-Timestamp"), r.Header.Get("X-Slack-Signature"),
		body, time.Now()); err != nil {
		h.Logger.Warn("slack command rejected", "err", err)
		writeError(w, r, http.StatusUnauthorized, CodeUnauthorized, "invalid slack signature")
		return
	}
	form, err := url.ParseQuery(string(body))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, CodeInvalidInput, "invalid form body")
		return
	}

	contractID := strings.ToUpper(strings.TrimSpace(form.Get("text")))
	if contractID == "" || contractID == "HELP" {
		writeJSON(w, http.StatusOK, slackEphemeral("Usage: `"+form.Get("command")+" <contract_id>` shows a contract's watchdog status and latest alerts."))
		return
	}

	m, err := h.Store.GetMonitoredContract(r.Context(), contractID)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusOK, slackEphemeral(fmt.Sprintf("`%s` is not monitored by the Sorolens watchdog.", contractID)))
		return
	}
	if err != nil {
		h.Logger.Error("slack command: get monitored contract", "err", err)
		writeJSON(w, http.StatusOK, slackEphemeral("Sorolens could not look up that contract right now. Try again shortly."))
		return
	}
	alerts, _, err := h.Store.ListAlerts(r.Context(), contractID, "", "", "", 3)
	if err != nil {
		h.Logger.Error("slack command: list alerts", "err", err)
	}
	writeJSON(w, http.StatusOK, slackStatusMessage(m, alerts))
}

func slackEphemeral(text string) map[string]any {
	return map[string]any{"response_type": "ephemeral", "text": text}
}

func slackStatusMessage(m store.MonitoredContract, alerts []store.ContractAlert) map[string]any {
	lastCheck := "never"
	if m.LastCheck != nil {
		lastCheck = m.LastCheck.UTC().Format(time.RFC3339)
	}
	blocks := []any{
		map[string]any{
			"type": "section",
			"text": map[string]any{"type": "mrkdwn", "text": fmt.Sprintf("*%s* is *%s*", m.Name, m.Status)},
			"fields": []any{
				map[string]any{"type": "mrkdwn", "text": fmt.Sprintf("*Contract*\n`%s`", m.ContractID)},
				map[string]any{"type": "mrkdwn", "text": fmt.Sprintf("*Last check*\n%s", lastCheck)},
			},
		},
	}
	if len(alerts) > 0 {
		lines := make([]string, len(alerts))
		for i, a := range alerts {
			lines[i] = fmt.Sprintf("• *%s* %s (%s)", a.Severity, a.Message, a.Timestamp.UTC().Format(time.RFC3339))
		}
		blocks = append(blocks, map[string]any{
			"type": "section",
			"text": map[string]any{"type": "mrkdwn", "text": "*Latest alerts*\n" + strings.Join(lines, "\n")},
		})
	}
	return map[string]any{
		"response_type": "ephemeral",
		"text":          fmt.Sprintf("%s is %s", m.Name, m.Status),
		"blocks":        blocks,
	}
}
