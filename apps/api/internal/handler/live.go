package handler

import (
	"net/http"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// ---- recent events (event ticker) -------------------------------------------

type recentEventsResponse struct {
	Events []eventResponse `json:"events"`
}

// RecentEvents handles GET /api/v1/events/recent.
//
// Returns the newest events across every tracked contract. The /live
// dashboard's ticker polls this endpoint every 5 seconds and de-duplicates by
// event id, so no server-side cursor is needed.
func (h *Handler) RecentEvents(w http.ResponseWriter, r *http.Request) {
	limit := intQuery(r, "limit", 50)
	if limit > 200 {
		limit = 200
	}
	events, err := h.Store.RecentEventsAll(r.Context(), limit)
	if err != nil {
		h.Logger.Error("recent events", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch recent events")
		return
	}
	resp := make([]eventResponse, len(events))
	for i, e := range events {
		resp[i] = eventFromStore(e)
	}
	writeJSON(w, http.StatusOK, recentEventsResponse{Events: resp})
}

// ---- per-contract activity (sparklines + leaderboard) -----------------------

type contractRateResponse struct {
	ContractID string  `json:"contract_id"`
	Label      string  `json:"label"`
	Network    string  `json:"network"`
	Total      int64   `json:"total"`
	PerMinute  []int64 `json:"per_minute"`
}

type activityResponse struct {
	Minutes     int                    `json:"minutes"`
	WindowStart time.Time              `json:"window_start"`
	Contracts   []contractRateResponse `json:"contracts"`
}

// LiveActivity handles GET /api/v1/stats/activity.
//
// Returns one entry per contract that emitted events in the last `minutes`
// minutes, ordered hottest first. Each entry carries a contiguous per-minute
// series (oldest first) so the sparklines and the "hot contracts" leaderboard
// render from a single request.
func (h *Handler) LiveActivity(w http.ResponseWriter, r *http.Request) {
	minutes := intQuery(r, "minutes", store.LiveWindowMinute)
	if minutes > store.MaxLiveWindowMinute {
		minutes = store.MaxLiveWindowMinute
	}

	rates, err := h.Store.ContractEventRates(r.Context(), minutes)
	if err != nil {
		h.Logger.Error("live activity", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch live activity")
		return
	}

	resp := make([]contractRateResponse, len(rates))
	for i, rate := range rates {
		resp[i] = contractRateResponse{
			ContractID: rate.ContractID,
			Label:      rate.Label,
			Network:    rate.Network,
			Total:      rate.Total,
			PerMinute:  rate.PerMinute,
		}
	}

	writeJSON(w, http.StatusOK, activityResponse{
		Minutes:     minutes,
		WindowStart: store.LiveWindowStart(minutes, time.Now()),
		Contracts:   resp,
	})
}
