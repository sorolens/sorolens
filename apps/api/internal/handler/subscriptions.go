package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// ---- request/response types ------------------------------------------------

type createSubscriptionRequest struct {
	ContractID     string `json:"contract_id"`
	WebhookURL     string `json:"webhook_url"`
	SeverityFilter string `json:"severity_filter"`
}

type subscriptionResponse struct {
	ID                 string     `json:"id"`
	ContractID         string     `json:"contract_id"`
	WebhookURL         string     `json:"webhook_url"`
	SeverityFilter     string     `json:"severity_filter"`
	LastDeliveryStatus *string    `json:"last_delivery_status,omitempty"`
	LastDeliveryAt     *time.Time `json:"last_delivery_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type subscriptionsResponse struct {
	Subscriptions []subscriptionResponse `json:"subscriptions"`
}

type deliveryHistoryResponse struct {
	Deliveries []store.WebhookDelivery `json:"deliveries"`
	Page       int                     `json:"page"`
	Limit      int                     `json:"limit"`
	Total      int                     `json:"total"`
}

// ---- converters -------------------------------------------------------------

func subscriptionFromStore(s store.AlertSubscription) subscriptionResponse {
	return subscriptionResponse{
		ID:                 s.ID,
		ContractID:         s.ContractID,
		WebhookURL:         s.WebhookURL,
		SeverityFilter:     s.SeverityFilter,
		LastDeliveryStatus: s.LastDeliveryStatus,
		LastDeliveryAt:     s.LastDeliveryAt,
		CreatedAt:          s.CreatedAt,
		UpdatedAt:          s.UpdatedAt,
	}
}

// ---- handlers ---------------------------------------------------------------

// CreateSubscription handles POST /api/v1/subscriptions and POST /api/v1/watchdog/subscriptions.
func (h *Handler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	var req createSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}
	if req.ContractID == "" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "contract_id is required")
		return
	}
	if req.WebhookURL == "" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "webhook_url is required")
		return
	}
	severity := req.SeverityFilter
	if severity == "" {
		severity = "Critical"
	}

	sub := store.AlertSubscription{
		ID:             fmt.Sprintf("sub_%d", time.Now().UnixNano()),
		ContractID:     req.ContractID,
		WebhookURL:     req.WebhookURL,
		SeverityFilter: severity,
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

// ListSubscriptions handles GET /api/v1/subscriptions and GET /api/v1/watchdog/subscriptions.
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

// DeleteSubscription handles DELETE /api/v1/subscriptions/:id and DELETE /api/v1/watchdog/subscriptions/:id.
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

// ListSubscriptionDeliveries handles GET /api/v1/subscriptions/:id/deliveries and GET /api/v1/watchdog/subscriptions/:id/deliveries.
func (h *Handler) ListSubscriptionDeliveries(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := h.Store.GetByID(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "subscription not found")
			return
		}
		h.Logger.Error("get alert subscription", "err", err, "id", id)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to get subscription")
		return
	}

	page := intQuery(r, "page", 1)
	limit := intQuery(r, "limit", 20)

	deliveries, total, err := h.Store.ListDeliveriesBySubscription(r.Context(), id, page, limit)
	if err != nil {
		h.Logger.Error("list subscription deliveries", "err", err, "id", id)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list subscription deliveries")
		return
	}

	if deliveries == nil {
		deliveries = []store.WebhookDelivery{}
	}

	writeJSON(w, http.StatusOK, deliveryHistoryResponse{
		Deliveries: deliveries,
		Page:       page,
		Limit:      limit,
		Total:      total,
	})
}
