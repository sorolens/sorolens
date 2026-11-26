package handler

import (
	"net/http"
	"strings"

	"github.com/sorolens/sorolens/apps/api/internal/cursor"
	"github.com/sorolens/sorolens/apps/api/internal/evfilter"
	"github.com/sorolens/sorolens/apps/api/internal/store"
	"github.com/sorolens/sorolens/apps/api/internal/timerange"
)

// SearchEventValues handles GET /api/v1/events/search?q=&cursor=&limit=.
//
// It searches decoded event topic and value data using the full-text index
// (issue #798), so a user can paste a contract address, token symbol or amount
// and find matching events.  Results are newest-first and cursor-paginated.
// The hard cap is 200 results per page; the default page size is 50.
func (h *Handler) SearchEventValues(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "q is required")
		return
	}
	rawCursor, ok := cursor.Decode(r.URL.Query().Get("cursor"))
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid cursor")
		return
	}

	events, nextRaw, err := h.Store.SearchEventValues(r.Context(), q, rawCursor, pageLimit(r, DefaultPageSize))
	if err != nil {
		h.Logger.Error("search event values", "err", err, "q", q)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to search events")
		return
	}

	resp := make([]eventResponse, len(events))
	for i, e := range events {
		resp[i] = eventFromStore(e)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"events":      resp,
		"next_cursor": cursor.Encode(nextRaw),
		"query":       q,
	})
}

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
	if filterExpr := strings.TrimSpace(q.Get("filter")); filterExpr != "" {
		expr, err := evfilter.Parse(filterExpr)
		if err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid filter expression: "+err.Error())
			return
		}
		if !expr.Empty() {
			// The base query uses $1–$6 for fixed predicates; filter args start
			// at $7. We pass a 6-element slice so AppendSQL generates $7, $8, …
			baseArgs := make([]any, 6)
			var sb strings.Builder
			expr.AppendSQL(&sb, &baseArgs)
			f.FilterSQL = sb.String()
			f.FilterArgs = baseArgs[6:] // only the newly appended args
		}
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
