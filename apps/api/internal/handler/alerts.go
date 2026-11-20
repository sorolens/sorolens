package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/middleware"
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

type snoozeRequest struct {
	Duration        string `json:"duration,omitempty"`
	DurationSeconds int64  `json:"duration_seconds,omitempty"`
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
//	state=<s>        — filter by state (open | acknowledged | snoozed | resolved)
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
	alerts, next, err := h.Store.ListAlertsWithState(
		r.Context(),
		q.Get("contract_id"),
		q.Get("severity"),
		network,
		q.Get("state"),
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

// AcknowledgeAlert handles POST /api/v1/alerts/{id}/ack (issue #789).
func (h *Handler) AcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid alert id")
		return
	}

	actor := middleware.Actor(r)
	alert, err := h.Store.AcknowledgeAlert(r.Context(), id, actor)
	if err != nil {
		if errors.Is(err, store.ErrAlertNotFound) || errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "alert not found")
			return
		}
		if errors.Is(err, store.ErrInvalidTransition) {
			writeError(w, r, http.StatusConflict, CodeConflict, err.Error())
			return
		}
		h.Logger.Error("ack alert", "id", id, "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to acknowledge alert")
		return
	}

	writeJSON(w, http.StatusOK, alertFromStore(alert))
}

// SnoozeAlert handles POST /api/v1/alerts/{id}/snooze (issue #789).
func (h *Handler) SnoozeAlert(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid alert id")
		return
	}

	var req snoozeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid request body")
		return
	}

	var dur time.Duration
	if req.Duration != "" {
		d, err := time.ParseDuration(req.Duration)
		if err != nil || d <= 0 {
			writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid snooze duration")
			return
		}
		dur = d
	} else if req.DurationSeconds > 0 {
		dur = time.Duration(req.DurationSeconds) * time.Second
	} else {
		dur = 1 * time.Hour
	}

	actor := middleware.Actor(r)
	alert, err := h.Store.SnoozeAlert(r.Context(), id, dur, actor)
	if err != nil {
		if errors.Is(err, store.ErrAlertNotFound) || errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "alert not found")
			return
		}
		if errors.Is(err, store.ErrInvalidTransition) {
			writeError(w, r, http.StatusConflict, CodeConflict, err.Error())
			return
		}
		h.Logger.Error("snooze alert", "id", id, "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to snooze alert")
		return
	}

	writeJSON(w, http.StatusOK, alertFromStore(alert))
}

// ResolveAlert handles POST /api/v1/alerts/{id}/resolve (issue #789).
func (h *Handler) ResolveAlert(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid alert id")
		return
	}

	actor := middleware.Actor(r)
	alert, err := h.Store.ResolveAlert(r.Context(), id, actor)
	if err != nil {
		if errors.Is(err, store.ErrAlertNotFound) || errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "alert not found")
			return
		}
		if errors.Is(err, store.ErrInvalidTransition) {
			writeError(w, r, http.StatusConflict, CodeConflict, err.Error())
			return
		}
		h.Logger.Error("resolve alert", "id", id, "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to resolve alert")
		return
	}

	writeJSON(w, http.StatusOK, alertFromStore(alert))
}

// AlertCounts handles GET /api/v1/alerts/counts (issue #789).
func (h *Handler) AlertCounts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	contractID := q.Get("contract_id")
	network := q.Get("network")
	counts, err := h.Store.GetAlertCountsByState(r.Context(), contractID, network)
	if err != nil {
		h.Logger.Error("alert counts", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch alert counts")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"counts": map[string]int64{
			store.AlertStateOpen:         counts[store.AlertStateOpen],
			store.AlertStateAcknowledged: counts[store.AlertStateAcknowledged],
			store.AlertStateSnoozed:      counts[store.AlertStateSnoozed],
			store.AlertStateResolved:     counts[store.AlertStateResolved],
		},
	})
}

