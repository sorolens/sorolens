package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// ---- response types --------------------------------------------------------

type alertGroupResponse struct {
	ID               int64     `json:"id"`
	GroupKey         string    `json:"group_key"`
	ContractID       string    `json:"contract_id"`
	Severity         string    `json:"severity"`
	Rule             string    `json:"rule"`
	Count            int64     `json:"count"`
	DedupeWindowSecs int64     `json:"dedupe_window_secs"`
	FirstSeen        time.Time `json:"first_seen"`
	LastSeen         time.Time `json:"last_seen"`
	LastMessage      string    `json:"last_message"`
	BackfillEligible bool      `json:"backfill_eligible"`
}

func alertGroupFromStore(g store.AlertGroup) alertGroupResponse {
	return alertGroupResponse{
		ID:               g.ID,
		GroupKey:         g.GroupKey,
		ContractID:       g.ContractID,
		Severity:         g.Severity,
		Rule:             g.Rule,
		Count:            g.Count,
		DedupeWindowSecs: g.DedupeWindowSecs,
		FirstSeen:        g.FirstSeen,
		LastSeen:         g.LastSeen,
		LastMessage:      g.LastMessage,
		BackfillEligible: g.BackfillEligible,
	}
}

// ---- handler ---------------------------------------------------------------

// ListAlerts handles GET /api/v1/alerts.
//
// By default it returns the grouped view (one row per deduplicated group),
// ordered by last_seen DESC with keyset pagination. Pass ?flat=true to get
// the raw ContractAlert feed from the existing watchdog store instead.
//
// Query parameters:
//
//	flat=true        — return raw alerts instead of grouped view
//	contract_id=<id> — filter by contract
//	severity=<s>     — filter by severity (Info | Warning | Critical)
//	network=<n>      — filter by network (testnet | mainnet | futurenet)
//	cursor=<c>       — continuation cursor from a previous page
//	limit=<n>        — page size (default 100, max 500)
func (h *Handler) ListAlerts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	flat := q.Get("flat") == "true"

	if flat {
		// Delegate to the existing raw alerts feed on the watchdog handler.
		h.listAlertsFlat(w, r)
		return
	}

	network, ok := networkParam(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput,
			"network must be one of: testnet, mainnet, futurenet, standalone")
		return
	}

	f := store.AlertGroupFilters{
		ContractID: q.Get("contract_id"),
		Severity:   q.Get("severity"),
		Network:    network,
	}

	groups, next, err := h.Store.ListAlertGroups(r.Context(), q.Get("cursor"), intQuery(r, "limit", 100), f)
	if err != nil {
		if errors.Is(err, store.ErrInvalidCursor) {
			writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid cursor")
			return
		}
		h.Logger.Error("list alert groups", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list alert groups")
		return
	}

	resp := make([]alertGroupResponse, len(groups))
	for i, g := range groups {
		resp[i] = alertGroupFromStore(g)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"groups":      resp,
		"next_cursor": next,
	})
}

// listAlertsFlat is the ?flat=true path: it reuses ListWatchdogAlerts logic
// directly by reading the same query params and delegating to the store.
func (h *Handler) listAlertsFlat(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	network, ok := networkParam(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput,
			"network must be one of: testnet, mainnet, futurenet, standalone")
		return
	}
	alerts, next, err := h.Store.ListAlerts(
		r.Context(),
		q.Get("contract_id"),
		q.Get("severity"),
		network,
		q.Get("cursor"),
		intQuery(r, "limit", 100),
	)
	if err != nil {
		if errors.Is(err, store.ErrInvalidCursor) {
			writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid cursor")
			return
		}
		h.Logger.Error("list alerts flat", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list alerts")
		return
	}
	resp := make([]contractAlertResponse, len(alerts))
	for i, a := range alerts {
		resp[i] = alertFromStore(a)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"alerts":      resp,
		"next_cursor": next,
	})
}
