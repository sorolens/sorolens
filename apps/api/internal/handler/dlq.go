package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

type failedEventResponse struct {
	ID           int64           `json:"id"`
	EventID      string          `json:"event_id"`
	ContractID   string          `json:"contract_id"`
	Network      string          `json:"network"`
	ErrorMessage string          `json:"error_message"`
	Attempts     int             `json:"attempts"`
	CreatedAt    string          `json:"created_at"`
	Event        json.RawMessage `json:"event,omitempty"`
}

type failedEventsListResponse struct {
	Items      []failedEventResponse `json:"items"`
	NextCursor string                `json:"next_cursor,omitempty"`
}

type requeueResponse struct {
	ID      int64  `json:"id"`
	EventID string `json:"event_id"`
	Status  string `json:"status"`
}

func failedEventFromStore(fe store.FailedEvent, includePayload bool) failedEventResponse {
	resp := failedEventResponse{
		ID:           fe.ID,
		EventID:      fe.EventID,
		ContractID:   fe.ContractID,
		Network:      fe.Network,
		ErrorMessage: fe.ErrorMessage,
		Attempts:     fe.Attempts,
		CreatedAt:    fe.CreatedAt.UTC().Format(time.RFC3339),
	}
	if includePayload {
		resp.Event = fe.EventPayload
	}
	return resp
}

// ListFailedEvents handles GET /api/v1/dlq.
// Returns dead-lettered events newest-first with opaque cursor pagination.
func (h *Handler) ListFailedEvents(w http.ResponseWriter, r *http.Request) {
	cursor := r.URL.Query().Get("cursor")
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 200 {
			writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "limit must be 1-200")
			return
		}
		limit = n
	}

	items, next, err := h.Store.ListFailedEvents(r.Context(), cursor, limit)
	if err != nil {
		h.Logger.Error("list failed events", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list DLQ")
		return
	}

	resp := failedEventsListResponse{
		Items:      make([]failedEventResponse, 0, len(items)),
		NextCursor: next,
	}
	for _, fe := range items {
		resp.Items = append(resp.Items, failedEventFromStore(fe, true))
	}
	writeJSON(w, http.StatusOK, resp)
}

// RequeueFailedEvent handles POST /api/v1/dlq/{id}/requeue.
// Re-inserts the parked event into the events table and clears the DLQ row
// on success (issue #202 acceptance: requeue re-inserts and clears the row).
func (h *Handler) RequeueFailedEvent(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id < 1 {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid DLQ id")
		return
	}

	fe, err := h.Store.GetFailedEvent(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "DLQ item not found")
			return
		}
		h.Logger.Error("get failed event", "err", err, "id", id)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load DLQ item")
		return
	}

	var ev store.Event
	if err := json.Unmarshal(fe.EventPayload, &ev); err != nil {
		h.Logger.Error("unmarshal failed event payload", "err", err, "id", id)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "corrupt DLQ payload")
		return
	}
	if ev.ID == "" {
		ev.ID = fe.EventID
	}
	if ev.ContractID == "" {
		ev.ContractID = fe.ContractID
	}
	if ev.Network == "" {
		ev.Network = fe.Network
	}

	if err := h.Store.BatchInsertEvents(r.Context(), []store.Event{ev}); err != nil {
		h.Logger.Error("requeue insert events", "err", err, "id", id, "event_id", fe.EventID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to re-insert event")
		return
	}

	if err := h.Store.DeleteFailedEvent(r.Context(), id); err != nil {
		h.Logger.Error("delete failed event after requeue", "err", err, "id", id)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "event inserted but failed to clear DLQ")
		return
	}

	writeJSON(w, http.StatusOK, requeueResponse{
		ID:      id,
		EventID: fe.EventID,
		Status:  "requeued",
	})
}
