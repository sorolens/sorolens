package handler

import (
	"net/http"
)

type globalStatsResponse struct {
	TrackedContracts    int64 `json:"tracked_contracts"`
	TotalEvents         int64 `json:"total_events"`
	TotalInvocations    int64 `json:"total_invocations"`
	TotalStorageEntries int64 `json:"total_storage_entries"`
}

// GlobalStats handles GET /api/v1/stats/global.
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
	})
}
