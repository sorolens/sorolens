package handler

import (
	"github.com/sorolens/sorolens/apps/api/internal/cursor"
	"net/http"
	"strings"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// validEventTypes are the Soroban event types accepted by the ?type= filter.
var validEventTypes = map[string]bool{
	"contract":   true,
	"system":     true,
	"diagnostic": true,
}

// ListAllEvents handles GET /api/v1/events: the cross-contract events
// explorer feed (issue #97), newest first.
//
// Query params: cursor, limit (default 50, clamped to MaxPageSize), contract_id
// (prefix match), type (contract | system | diagnostic), network, since and
// until (RFC 3339, inclusive bounds on ledger_closed_at). A malformed cursor,
// network, type or date is rejected with its own 422 code (issue #776).
func (h *Handler) ListAllEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	rawCursor, ok := decodeCursor(w, r)
	if !ok {
		return
	}
	network, ok := networkFilter(w, r)
	if !ok {
		return
	}
	eventType, ok := enumParam(w, r, "type", validEventTypes, "type must be one of: contract, system, diagnostic")
	if !ok {
		return
	}
	f := store.GlobalEventFilters{
		ContractID: strings.ToUpper(strings.TrimSpace(q.Get("contract_id"))),
		Type:       eventType,
		Network:    network,
	}
	for _, p := range []struct {
		key string
		dst *time.Time
	}{{"since", &f.Since}, {"until", &f.Until}} {
		v := strings.TrimSpace(q.Get(p.key))
		if v == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidDate, p.key+" must be an RFC 3339 timestamp")
			return
		}
		*p.dst = t
	}
	if !f.Since.IsZero() && !f.Until.IsZero() && f.Until.Before(f.Since) {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidDate, "until must not be before since")
		return
	}
	limit, ok := pageLimit(w, r, 50)
	if !ok {
		return
	}

	events, nextRaw, err := h.Store.ListAllEvents(r.Context(), rawCursor, limit, f)
	if err != nil {
		h.Logger.Error("list all events", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list events")
		return
	}
	resp := make([]eventResponse, len(events))
	for i, e := range events {
		resp[i] = eventFromStore(e)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"events":      resp,
		"next_cursor": cursor.Encode(nextRaw),
	})
}
