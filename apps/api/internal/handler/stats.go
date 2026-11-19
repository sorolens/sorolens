package handler

import (
	"net/http"
)

type globalStatsResponse struct {
	TrackedContracts    int64 `json:"tracked_contracts"`
	TotalEvents         int64 `json:"total_events"`
	TotalInvocations    int64 `json:"total_invocations"`
	TotalStorageEntries int64 `json:"total_storage_entries"`
	// Approximate reports that at least one total is the planner's row
	// estimate rather than an exact count (issue #776). Callers should label
	// the numbers ("~12.4M events") when it is true instead of presenting them
	// as exact.
	Approximate bool `json:"approximate"`
}

// GlobalStats handles GET /api/v1/stats/global.
//
// The response is cached for CacheTTL (the same short window as the contracts
// list) so a dashboard refreshing this endpoint does not re-run the aggregate
// query per poll, and the store itself reads planner estimates for the
// high-volume tables instead of counting them (issue #776).
func (h *Handler) GlobalStats(w http.ResponseWriter, r *http.Request) {
	gs, err := h.Store.GetGlobalStats(r.Context())
	if err != nil {
		h.Logger.Error("get global stats", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch stats")
		return
	}
	writeJSON(w, http.StatusOK, globalStatsResponse{
		TrackedContracts:    gs.TrackedContracts,
		TotalEvents:         gs.TotalEvents,
		TotalInvocations:    gs.TotalInvocations,
		TotalStorageEntries: gs.TotalStorageEntries,
		Approximate:         gs.Approximate,
	})
}
