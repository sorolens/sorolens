package handler

import (
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
// Query params: cursor, limit (default 50, max 200), contract_id (prefix
// match), type (contract | system | diagnostic), network, since and until
// (RFC 3339, inclusive bounds on ledger_closed_at).
func (h *Handler) ListAllEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	rawCursor, ok := decodeCursor(q.Get("cursor"))
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid cursor")
		return
	}
	network, ok := networkParam(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "network must be one of: testnet, mainnet, futurenet, standalone")
		return
	}
	f := store.GlobalEventFilters{
		ContractID: strings.ToUpper(strings.TrimSpace(q.Get("contract_id"))),
		Type:       strings.TrimSpace(q.Get("type")),
		Network:    network,
	}
	if f.Type != "" && !validEventTypes[f.Type] {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "type must be one of: contract, system, diagnostic")
		return
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
			writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, p.key+" must be an RFC 3339 timestamp")
			return
		}
		*p.dst = t
	}
	if !f.Since.IsZero() && !f.Until.IsZero() && f.Until.Before(f.Since) {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "until must not be before since")
		return
	}

	events, nextRaw, err := h.Store.ListAllEvents(r.Context(), rawCursor, intQuery(r, "limit", 50), f)
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
		"next_cursor": encodeCursor(nextRaw),
	})
}
