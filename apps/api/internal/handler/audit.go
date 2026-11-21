package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/cursor"
	"github.com/sorolens/sorolens/apps/api/internal/store"
	"github.com/sorolens/sorolens/apps/api/internal/timerange"
)

type auditEventResponse struct {
	ID              int64     `json:"id"`
	Actor           string    `json:"actor"`
	Action          string    `json:"action"`
	ResourceType    string    `json:"resource_type"`
	ResourceID      string    `json:"resource_id"`
	IP              string    `json:"ip"`
	UserAgent       string    `json:"user_agent"`
	RequestBodyHash string    `json:"request_body_hash"`
	Status          int       `json:"status"`
	At              time.Time `json:"at"`
}

func auditEventFromStore(e store.AuditEvent) auditEventResponse {
	return auditEventResponse{
		ID:              e.ID,
		Actor:           e.Actor,
		Action:          e.Action,
		ResourceType:    e.ResourceType,
		ResourceID:      e.ResourceID,
		IP:              e.IP,
		UserAgent:       e.UserAgent,
		RequestBodyHash: e.RequestBodyHash,
		Status:          e.Status,
		At:              e.At,
	}
}

// ListAuditEvents handles GET /api/v1/admin/audit (admin only).
//
// Query params: from and to (RFC 3339, inclusive), limit (default 100, max 500),
// cursor (opaque, from next_cursor). The deprecated since parameter remains
// supported as an alias for from. Results are newest first.
func (h *Handler) ListAuditEvents(w http.ResponseWriter, r *http.Request) {
	tr, err := timerange.ParseRequest(r, timerange.DefaultOptions())
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, err.Error())
		return
	}

	rawCursor, ok := cursor.Decode(r.URL.Query().Get("cursor"))
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid cursor")
		return
	}
	var auditCursor int64
	if rawCursor != "" {
		n, err := strconv.ParseInt(rawCursor, 10, 64)
		if err != nil || n <= 0 {
			writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid cursor")
			return
		}
		auditCursor = n
	}

	events, next, err := h.Store.ListAuditEvents(r.Context(), tr.From, tr.To, auditCursor, intQuery(r, "limit", 100))
	if err != nil {
		h.Logger.Error("list audit events", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list audit events")
		return
	}

	resp := make([]auditEventResponse, len(events))
	for i, e := range events {
		resp[i] = auditEventFromStore(e)
	}
	var nextCursor string
	if next != 0 {
		nextCursor = cursor.Encode(strconv.FormatInt(next, 10))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"events":      resp,
		"next_cursor": nextCursor,
	})
}
