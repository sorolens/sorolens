package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// ---- request/response types ------------------------------------------------

type createSubscriptionRequest struct {
	ContractID     string `json:"contract_id"`
	WebhookURL     string `json:"webhook_url"`
	SeverityFilter string `json:"severity_filter"`
	// ChannelType is webhook (default) | slack | discord | pagerduty.
	ChannelType string `json:"channel_type"`
	// RoutingKey is the PagerDuty integration key (pagerduty only).
	RoutingKey string `json:"routing_key"`
}

// subscriptionResponse never echoes integration secrets: Slack and Discord
// webhook URLs embed a token, so they are masked, and the PagerDuty routing
// key is reported only as present or absent.
type subscriptionResponse struct {
	ID             string    `json:"id"`
	ContractID     string    `json:"contract_id"`
	ChannelType    string    `json:"channel_type"`
	WebhookURL     string    `json:"webhook_url"`
	HasRoutingKey  bool      `json:"has_routing_key"`
	SeverityFilter string    `json:"severity_filter"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Notification channel types (issue #127).
const (
	channelWebhook   = "webhook"
	channelSlack     = "slack"
	channelDiscord   = "discord"
	channelPagerDuty = "pagerduty"
)

var validChannels = map[string]bool{
	channelWebhook: true, channelSlack: true, channelDiscord: true, channelPagerDuty: true,
}

var validSeverities = map[string]bool{"Info": true, "Warning": true, "Critical": true}

// maskSecretURL keeps the scheme, host and first path segment of a
// credential-bearing webhook URL and hides the rest.
func maskSecretURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "***"
	}
	segs := strings.SplitN(strings.TrimPrefix(u.Path, "/"), "/", 2)
	out := u.Scheme + "://" + u.Host
	if segs[0] != "" {
		out += "/" + segs[0]
	}
	return out + "/***"
}

// validateSubscription normalizes req and checks the channel-specific
// requirements. It returns a client-facing error message, or "".
func validateSubscription(req *createSubscriptionRequest) string {
	req.ChannelType = strings.ToLower(strings.TrimSpace(req.ChannelType))
	if req.ChannelType == "" {
		req.ChannelType = channelWebhook
	}
	if !validChannels[req.ChannelType] {
		return "channel_type must be one of: webhook, slack, discord, pagerduty"
	}
	if req.ContractID == "" {
		return "contract_id is required"
	}
	if req.SeverityFilter == "" {
		req.SeverityFilter = "Critical"
	}
	if !validSeverities[req.SeverityFilter] {
		return "severity_filter must be one of: Info, Warning, Critical"
	}
	req.WebhookURL = strings.TrimSpace(req.WebhookURL)
	req.RoutingKey = strings.TrimSpace(req.RoutingKey)

	switch req.ChannelType {
	case channelPagerDuty:
		if req.RoutingKey == "" {
			return "routing_key is required for pagerduty"
		}
		// webhook_url is optional: it defaults to the Events API v2 endpoint.
		if req.WebhookURL != "" && !isHTTPSURL(req.WebhookURL) {
			return "webhook_url must be an https URL"
		}
	case channelSlack, channelDiscord:
		if !isHTTPSURL(req.WebhookURL) {
			return "webhook_url must be the " + req.ChannelType + " incoming webhook (https) URL"
		}
	default:
		u, err := url.Parse(req.WebhookURL)
		if req.WebhookURL == "" || err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return "webhook_url must be an http(s) URL"
		}
	}
	if req.ChannelType != channelPagerDuty && req.RoutingKey != "" {
		return "routing_key is only valid for pagerduty"
	}
	return ""
}

func isHTTPSURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != ""
}

type subscriptionsResponse struct {
	Subscriptions []subscriptionResponse `json:"subscriptions"`
}

// ---- converters -------------------------------------------------------------

func subscriptionFromStore(s store.AlertSubscription) subscriptionResponse {
	channel := s.ChannelType
	if channel == "" {
		channel = channelWebhook
	}
	webhookURL := s.WebhookURL
	if channel != channelWebhook && webhookURL != "" {
		webhookURL = maskSecretURL(webhookURL)
	}
	return subscriptionResponse{
		ID:             s.ID,
		ContractID:     s.ContractID,
		ChannelType:    channel,
		WebhookURL:     webhookURL,
		HasRoutingKey:  s.RoutingKey != "",
		SeverityFilter: s.SeverityFilter,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
	}
}

// ---- handlers ---------------------------------------------------------------

// CreateSubscription handles POST /api/v1/watchdog/subscriptions.
func (h *Handler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	var req createSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}
	if msg := validateSubscription(&req); msg != "" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, msg)
		return
	}
	// Subscriptions reference monitored_contracts, so an unmonitored
	// contract is a client error rather than a foreign-key failure.
	if _, err := h.Store.GetMonitoredContract(r.Context(), req.ContractID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "contract_id is not monitored by the watchdog")
			return
		}
		h.Logger.Error("create alert subscription: get monitored contract", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to create subscription")
		return
	}

	sub := store.AlertSubscription{
		ID:             fmt.Sprintf("sub_%d", time.Now().UnixNano()),
		ContractID:     req.ContractID,
		WebhookURL:     req.WebhookURL,
		SeverityFilter: req.SeverityFilter,
		ChannelType:    req.ChannelType,
		RoutingKey:     req.RoutingKey,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	if err := h.Store.Create(r.Context(), sub); err != nil {
		h.Logger.Error("create alert subscription", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to create subscription")
		return
	}
	writeJSON(w, http.StatusCreated, subscriptionFromStore(sub))
}

// ListSubscriptions handles GET /api/v1/watchdog/subscriptions.
func (h *Handler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	subs, err := h.Store.ListAll(r.Context())
	if err != nil {
		h.Logger.Error("list alert subscriptions", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list subscriptions")
		return
	}
	resp := make([]subscriptionResponse, len(subs))
	for i, s := range subs {
		resp[i] = subscriptionFromStore(s)
	}
	writeJSON(w, http.StatusOK, map[string]any{"subscriptions": resp})
}

// DeleteSubscription handles DELETE /api/v1/watchdog/subscriptions/:id.
func (h *Handler) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.Store.Delete(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "subscription not found")
			return
		}
		h.Logger.Error("delete alert subscription", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to delete subscription")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
