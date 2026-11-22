package handler

import (
	"net/http"
	"strings"

	"github.com/sorolens/sorolens/apps/api/internal/cursor"
	"github.com/sorolens/sorolens/apps/api/internal/store"
	"github.com/sorolens/sorolens/apps/api/internal/timerange"
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
// match), type (contract | system | diagnostic), network, from and to
// (RFC 3339, inclusive bounds on ledger_closed_at). The deprecated since
// and until parameters remain supported as aliases for from and to.
func (h *Handler) ListAllEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	rawCursor, ok := cursor.Decode(q.Get("cursor"))
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid cursor")
		return
	}
	network, ok := networkParam(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "network must be one of: testnet, mainnet, futurenet, standalone")
		return
	}
	tr, err := timerange.ParseRequest(r, timerange.DefaultOptions())
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, err.Error())
		return
	}
	f := store.GlobalEventFilters{
		ContractID: strings.ToUpper(strings.TrimSpace(q.Get("contract_id"))),
		Type:       strings.TrimSpace(q.Get("type")),
		Network:    network,
		Since:      tr.From,
		Until:      tr.To,
	}
	if f.Type != "" && !validEventTypes[f.Type] {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "type must be one of: contract, system, diagnostic")
		return
	}

	events, nextRaw, err := h.Store.ListAllEvents(r.Context(), rawCursor, pageLimit(r, DefaultPageSize), f)
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
