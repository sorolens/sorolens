package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/cursor"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// ---- response types --------------------------------------------------------

type deliveryResponse struct {
	ID             string           `json:"id"`
	SubscriptionID string           `json:"subscription_id"`
	AttemptNumber  int              `json:"attempt_number"`
	Status         string           `json:"status"`
	SignedPayload  *json.RawMessage `json:"signed_payload,omitempty"`
	ResponseStatus *int             `json:"response_status,omitempty"`
	ResponseBody   *string          `json:"response_body,omitempty"`
	LastError      *string          `json:"last_error,omitempty"`
	NextRetryAt    *time.Time       `json:"next_retry_at,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

func deliveryFromStore(d store.WebhookDelivery, includePayload bool) deliveryResponse {
	resp := deliveryResponse{
		ID:             d.ID,
		SubscriptionID: d.SubscriptionID,
		AttemptNumber:  d.AttemptNumber,
		Status:         d.Status,
		ResponseStatus: d.ResponseStatus,
		ResponseBody:   d.ResponseBody,
		LastError:      d.LastError,
		NextRetryAt:    d.NextRetryAt,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}
	if includePayload && len(d.SignedPayload) > 0 {
		raw := json.RawMessage(d.SignedPayload)
		resp.SignedPayload = &raw
	}
	return resp
}

// ---- handlers --------------------------------------------------------------

// ListDeliveries handles GET /api/v1/watchdog/subscriptions/{id}/deliveries.
//
// Returns webhook delivery attempts for a subscription, newest first, cursor-
// paginated. The signed payload is omitted from list responses; use
// GetDelivery for the full record (issue #828).
func (h *Handler) ListDeliveries(w http.ResponseWriter, r *http.Request) {
	subID := chi.URLParam(r, "id")
	rawCursor, ok := cursor.Decode(r.URL.Query().Get("cursor"))
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid cursor")
		return
	}
	deliveries, nextRaw, err := h.Store.ListDeliveries(r.Context(), subID, rawCursor, pageLimit(r, DefaultPageSize))
	if err != nil {
		h.Logger.Error("list deliveries", "err", err, "subscription_id", subID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list deliveries")
		return
	}
	resp := make([]deliveryResponse, len(deliveries))
	for i, d := range deliveries {
		resp[i] = deliveryFromStore(d, false)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"deliveries":  resp,
		"next_cursor": cursor.Encode(nextRaw),
	})
}

// GetDelivery handles GET /api/v1/watchdog/subscriptions/{id}/deliveries/{deliveryID}.
//
// Returns the full delivery record including the signed payload (secrets in
// the HMAC signature header are not echoed; only the body is included) and the
// cURL command to replay the request (issue #828).
func (h *Handler) GetDelivery(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "deliveryID")
	d, err := h.Store.GetDelivery(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "delivery not found")
			return
		}
		h.Logger.Error("get delivery", "err", err, "id", id)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to get delivery")
		return
	}
	// Verify the delivery belongs to the subscription in the URL.
	subID := chi.URLParam(r, "id")
	if d.SubscriptionID != subID {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "delivery not found")
		return
	}
	writeJSON(w, http.StatusOK, deliveryFromStore(d, true))
}

// RetryDelivery handles POST /api/v1/watchdog/subscriptions/{id}/deliveries/{deliveryID}/retry.
//
// Marks a failed delivery for immediate retry by resetting its next_retry_at
// to now and incrementing the attempt counter. Only contributor+ callers may
// trigger a retry (issue #828). Returns the updated delivery.
func (h *Handler) RetryDelivery(w http.ResponseWriter, r *http.Request) {
	subID := chi.URLParam(r, "id")
	id := chi.URLParam(r, "deliveryID")

	// Check everything before any write: the subscription must exist and the
	// delivery must belong to it. Anything else is a 404 so ids belonging to
	// other subscriptions are not disclosed or modified.
	if _, err := h.Store.GetSubscription(r.Context(), subID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "delivery not found")
			return
		}
		h.Logger.Error("retry delivery: get subscription", "err", err, "subscription_id", subID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to retry delivery")
		return
	}
	existing, err := h.Store.GetDelivery(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "delivery not found")
			return
		}
		h.Logger.Error("retry delivery: get delivery", "err", err, "id", id)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to retry delivery")
		return
	}
	if existing.SubscriptionID != subID {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "delivery not found")
		return
	}

	// The store update is itself scoped to the subscription, closing the gap
	// between the check above and the write.
	d, err := h.Store.RetryDelivery(r.Context(), subID, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "delivery not found")
			return
		}
		h.Logger.Error("retry delivery", "err", err, "id", id)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to retry delivery")
		return
	}
	writeJSON(w, http.StatusAccepted, deliveryFromStore(d, false))
}
